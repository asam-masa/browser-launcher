[CmdletBinding()]
param(
    [ValidateRange(1, 300)]
    [int]$ObserveSeconds = 30,
    [ValidateRange(100, 5000)]
    [int]$PollIntervalMilliseconds = 1000,
    [ValidateRange(0, 300)]
    [int]$CancelAfterSeconds = 0,
    [ValidateRange(0, 300)]
    [int]$TimeoutAfterSeconds = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

if ($CancelAfterSeconds -ge $ObserveSeconds) {
    throw 'CancelAfterSeconds must be 0 or less than ObserveSeconds.'
}
if ($TimeoutAfterSeconds -ge $ObserveSeconds) {
    throw 'TimeoutAfterSeconds must be 0 or less than ObserveSeconds.'
}

Add-Type -TypeDefinition @'
using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;

public sealed class WindowEventObserver : IDisposable
{
    public sealed class RawEvent
    {
        public uint EventType { get; set; }
        public long WindowHandle { get; set; }
        public int ObjectId { get; set; }
        public int ChildId { get; set; }
        public uint EventThreadId { get; set; }
        public uint EventTimeMilliseconds { get; set; }
        public DateTime ReceivedAtUtc { get; set; }
    }

    private delegate void WinEventDelegate(
        IntPtr hook,
        uint eventType,
        IntPtr hwnd,
        int objectId,
        int childId,
        uint eventThreadId,
        uint eventTimeMilliseconds);

    [StructLayout(LayoutKind.Sequential)]
    private struct Point
    {
        public int X;
        public int Y;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct Message
    {
        public IntPtr WindowHandle;
        public uint Value;
        public IntPtr WParam;
        public IntPtr LParam;
        public uint Time;
        public Point Position;
    }

    private const uint EventObjectCreate = 0x8000;
    private const uint EventObjectShow = 0x8002;
    private const uint WineventOutOfContext = 0x0000;
    private const uint WineventSkipOwnProcess = 0x0002;
    private const uint WmQuit = 0x0012;

    private readonly ConcurrentQueue<RawEvent> events = new ConcurrentQueue<RawEvent>();
    private readonly ManualResetEventSlim ready = new ManualResetEventSlim(false);
    private readonly AutoResetEvent eventReceived = new AutoResetEvent(false);
    private readonly Thread thread;
    private WinEventDelegate callback;
    private GCHandle callbackHandle;
    private IntPtr hook;
    private uint threadId;
    private volatile Exception workerException;
    private bool disposed;

    public bool HookRegistered { get; private set; }
    public bool UnhookSucceeded { get; private set; }

    public WindowEventObserver()
    {
        thread = new Thread(Run);
        thread.IsBackground = true;
        thread.Name = "Chrome window event observer";
    }

    public void Start()
    {
        thread.Start();
        if (!ready.Wait(TimeSpan.FromSeconds(5)))
        {
            throw new InvalidOperationException("The WinEvent observer did not start in time.");
        }
        if (workerException != null)
        {
            throw new InvalidOperationException("The WinEvent observer failed to start.", workerException);
        }
    }

    public bool WaitForEvent(int millisecondsTimeout)
    {
        bool signaled = eventReceived.WaitOne(millisecondsTimeout);
        if (workerException != null)
        {
            throw new InvalidOperationException("The WinEvent observer failed.", workerException);
        }
        return signaled;
    }

    public RawEvent[] Drain()
    {
        List<RawEvent> drained = new List<RawEvent>();
        RawEvent item;
        while (events.TryDequeue(out item))
        {
            drained.Add(item);
        }
        return drained.ToArray();
    }

    public void Dispose()
    {
        if (disposed)
        {
            return;
        }
        disposed = true;

        if (threadId != 0 && thread.IsAlive)
        {
            PostThreadMessage(threadId, WmQuit, IntPtr.Zero, IntPtr.Zero);
            if (!thread.Join(TimeSpan.FromSeconds(5)))
            {
                throw new InvalidOperationException("The WinEvent observer did not stop in time.");
            }
        }
        ready.Dispose();
        eventReceived.Dispose();
    }

    private void Run()
    {
        try
        {
            threadId = GetCurrentThreadId();
            Message message;
            PeekMessage(out message, IntPtr.Zero, 0, 0, 0);

            callback = HandleEvent;
            callbackHandle = GCHandle.Alloc(callback);
            hook = SetWinEventHook(
                EventObjectCreate,
                EventObjectShow,
                IntPtr.Zero,
                callback,
                0,
                0,
                WineventOutOfContext | WineventSkipOwnProcess);
            if (hook == IntPtr.Zero)
            {
                throw new InvalidOperationException("SetWinEventHook failed.");
            }
            HookRegistered = true;

            ready.Set();
            while (true)
            {
                int result = GetMessage(out message, IntPtr.Zero, 0, 0);
                if (result == -1)
                {
                    throw new InvalidOperationException("GetMessage failed.");
                }
                if (result == 0)
                {
                    break;
                }
                TranslateMessage(ref message);
                DispatchMessage(ref message);
            }
        }
        catch (Exception exception)
        {
            workerException = exception;
            ready.Set();
            eventReceived.Set();
        }
        finally
        {
            if (hook != IntPtr.Zero)
            {
                UnhookSucceeded = UnhookWinEvent(hook);
                hook = IntPtr.Zero;
            }
            if (callbackHandle.IsAllocated)
            {
                callbackHandle.Free();
            }
            callback = null;
        }
    }

    private void HandleEvent(
        IntPtr eventHook,
        uint eventType,
        IntPtr hwnd,
        int objectId,
        int childId,
        uint eventThreadId,
        uint eventTimeMilliseconds)
    {
        events.Enqueue(new RawEvent
        {
            EventType = eventType,
            WindowHandle = hwnd.ToInt64(),
            ObjectId = objectId,
            ChildId = childId,
            EventThreadId = eventThreadId,
            EventTimeMilliseconds = eventTimeMilliseconds,
            ReceivedAtUtc = DateTime.UtcNow
        });
        eventReceived.Set();
    }

    [DllImport("user32.dll")]
    private static extern IntPtr SetWinEventHook(
        uint eventMin,
        uint eventMax,
        IntPtr module,
        WinEventDelegate callback,
        uint processId,
        uint threadId,
        uint flags);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool UnhookWinEvent(IntPtr hook);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool PostThreadMessage(
        uint threadId,
        uint message,
        IntPtr wParam,
        IntPtr lParam);

    [DllImport("user32.dll")]
    private static extern int GetMessage(
        out Message message,
        IntPtr windowHandle,
        uint filterMin,
        uint filterMax);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool PeekMessage(
        out Message message,
        IntPtr windowHandle,
        uint filterMin,
        uint filterMax,
        uint removeMessage);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool TranslateMessage(ref Message message);

    [DllImport("user32.dll")]
    private static extern IntPtr DispatchMessage(ref Message message);

    [DllImport("kernel32.dll")]
    private static extern uint GetCurrentThreadId();
}

public static class ChromeWindowSnapshot
{
    public delegate bool EnumWindowsDelegate(IntPtr hwnd, IntPtr lParam);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool EnumWindows(EnumWindowsDelegate callback, IntPtr lParam);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool IsWindow(IntPtr hwnd);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool IsWindowVisible(IntPtr hwnd);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int GetClassName(IntPtr hwnd, StringBuilder className, int maxCount);

    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint processId);
}
'@

$eventNames = @{
    0x8000 = 'Create'
    0x8001 = 'Destroy'
    0x8002 = 'Show'
}

function Get-ChromeWindowSnapshot {
    $windows = [System.Collections.Generic.List[object]]::new()
    $callback = [ChromeWindowSnapshot+EnumWindowsDelegate]{
        param([IntPtr]$hwnd, [IntPtr]$lParam)

        $className = [Text.StringBuilder]::new(256)
        [void][ChromeWindowSnapshot]::GetClassName($hwnd, $className, $className.Capacity)
        if ($className.ToString() -ne 'Chrome_WidgetWin_1') {
            return $true
        }

        [uint32]$processId = 0
        [void][ChromeWindowSnapshot]::GetWindowThreadProcessId($hwnd, [ref]$processId)
        try {
            $process = Get-Process -Id $processId -ErrorAction Stop
            if ($process.ProcessName -ne 'chrome') {
                return $true
            }

            $windows.Add([ordered]@{
                Handle = $hwnd.ToInt64()
                ProcessId = $processId
                ProcessStartTimeUtcTicks = $process.StartTime.ToUniversalTime().Ticks
                ClassName = $className.ToString()
                IsVisible = [ChromeWindowSnapshot]::IsWindowVisible($hwnd)
            })
        }
        catch {
            # The process may end between enumeration and identity lookup.
        }
        return $true
    }

    if (-not [ChromeWindowSnapshot]::EnumWindows($callback, [IntPtr]::Zero)) {
        throw 'EnumWindows failed.'
    }
    return @($windows)
}

function Convert-RawEvent {
    param([Parameter(Mandatory)]$Event)

    return [ordered]@{
        EventName = $eventNames[[int]$Event.EventType]
        EventType = ('0x{0:X4}' -f $Event.EventType)
        Handle = $Event.WindowHandle
        ObjectId = $Event.ObjectId
        ChildId = $Event.ChildId
        EventThreadId = $Event.EventThreadId
        EventTimeMilliseconds = $Event.EventTimeMilliseconds
        ReceivedAtUtc = $Event.ReceivedAtUtc.ToString('o')
    }
}

$observer = [WindowEventObserver]::new()
$records = [System.Collections.Generic.List[object]]::new()
$outcome = 'Completed'

try {
    $observer.Start()
    $startedAt = [DateTime]::UtcNow
    $deadline = $startedAt.AddSeconds($ObserveSeconds)
    $initialWindows = @(Get-ChromeWindowSnapshot)
    $lastSnapshotSignature = $initialWindows | ConvertTo-Json -Compress

    $records.Add([ordered]@{
        Reason = 'Initial'
        ObservedAtUtc = $startedAt.ToString('o')
        Events = @()
        ChromeWindows = $initialWindows
    })

    while ([DateTime]::UtcNow -lt $deadline) {
        [void]$observer.WaitForEvent($PollIntervalMilliseconds)
        if (
            $CancelAfterSeconds -gt 0 -and
            [DateTime]::UtcNow -ge $startedAt.AddSeconds($CancelAfterSeconds)
        ) {
            $outcome = 'Cancelled'
            break
        }
        if (
            $TimeoutAfterSeconds -gt 0 -and
            [DateTime]::UtcNow -ge $startedAt.AddSeconds($TimeoutAfterSeconds)
        ) {
            $outcome = 'TimedOut'
            break
        }

        $events = @($observer.Drain())
        $relevantEvents = @(
            $events | Where-Object {
                $_.ObjectId -eq 0 -and $_.ChildId -eq 0
            } | ForEach-Object { Convert-RawEvent -Event $_ }
        )

        $windows = @(Get-ChromeWindowSnapshot)
        $snapshotSignature = $windows | ConvertTo-Json -Compress
        if ($relevantEvents.Count -gt 0 -or $snapshotSignature -ne $lastSnapshotSignature) {
            $reason = if ($relevantEvents.Count -gt 0) {
                'WinEventAndPoll'
            }
            else {
                'SnapshotChanged'
            }
            $records.Add([ordered]@{
                Reason = $reason
                ObservedAtUtc = [DateTime]::UtcNow.ToString('o')
                Events = $relevantEvents
                ChromeWindows = $windows
            })
        }
        $lastSnapshotSignature = $snapshotSignature
    }
}
finally {
    $observer.Dispose()
}

[ordered]@{
    ObserveSeconds = $ObserveSeconds
    PollIntervalMilliseconds = $PollIntervalMilliseconds
    CancelAfterSeconds = $CancelAfterSeconds
    TimeoutAfterSeconds = $TimeoutAfterSeconds
    Outcome = $outcome
    HookRegistered = $observer.HookRegistered
    UnhookSucceeded = $observer.UnhookSucceeded
    Records = $records
} | ConvertTo-Json -Depth 8

if ($observer.HookRegistered -and -not $observer.UnhookSucceeded) {
    throw 'UnhookWinEvent failed.'
}
