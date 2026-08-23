[CmdletBinding()]
param(
    [switch]$Launch,
    [switch]$ApplyPosition,
    [ValidateRange(0, 32767)]
    [int]$X = 100,
    [ValidateRange(0, 32767)]
    [int]$Y = 100,
    [ValidateRange(1, 32767)]
    [int]$Width = 1000,
    [ValidateRange(1, 32767)]
    [int]$Height = 700,
    [ValidateRange(1, 60)]
    [int]$TimeoutSeconds = 15,
    [ValidateRange(0, 60)]
    [int]$ProfileSelectionWaitSeconds = 0
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

Add-Type -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using System.Text;

public static class WindowProbe
{
    public delegate bool EnumWindowsProc(IntPtr hwnd, IntPtr lParam);

    [StructLayout(LayoutKind.Sequential)]
    public struct Rect
    {
        public int Left;
        public int Top;
        public int Right;
        public int Bottom;
    }

    [StructLayout(LayoutKind.Sequential)]
    public struct Point
    {
        public int X;
        public int Y;
    }

    [StructLayout(LayoutKind.Sequential, CharSet = CharSet.Unicode)]
    public struct MonitorInfo
    {
        public int Size;
        public Rect Monitor;
        public Rect Work;
        public uint Flags;
    }

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool EnumWindows(EnumWindowsProc callback, IntPtr lParam);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool IsWindowVisible(IntPtr hwnd);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool IsWindow(IntPtr hwnd);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    public static extern int GetClassName(IntPtr hwnd, StringBuilder className, int maxCount);

    [DllImport("user32.dll")]
    public static extern uint GetWindowThreadProcessId(IntPtr hwnd, out uint processId);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool GetWindowRect(IntPtr hwnd, out Rect rect);

    [DllImport("user32.dll")]
    public static extern uint GetDpiForWindow(IntPtr hwnd);

    [DllImport("user32.dll")]
    public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr dpiContext);

    [DllImport("user32.dll")]
    public static extern IntPtr MonitorFromPoint(Point point, uint flags);

    [DllImport("user32.dll")]
    public static extern IntPtr MonitorFromWindow(IntPtr hwnd, uint flags);

    [DllImport("user32.dll", CharSet = CharSet.Unicode)]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool GetMonitorInfo(IntPtr monitor, ref MonitorInfo info);

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    public static extern bool SetWindowPos(
        IntPtr hwnd,
        IntPtr insertAfter,
        int x,
        int y,
        int width,
        int height,
        uint flags
    );

    [DllImport("dwmapi.dll")]
    public static extern int DwmGetWindowAttribute(
        IntPtr hwnd,
        int attribute,
        out Rect value,
        int size
    );
}
'@

$dpiContextPerMonitorAwareV2 = [IntPtr]::new(-4)
$previousDpiContext = [WindowProbe]::SetThreadDpiAwarenessContext($dpiContextPerMonitorAwareV2)
if ($previousDpiContext -eq [IntPtr]::Zero) {
    throw 'Failed to enable the Per-Monitor DPI Aware V2 context.'
}

function Get-ChromeExecutable {
    $allowedCandidates = @(
        @{ Path = Join-Path $env:LOCALAPPDATA 'Google\Chrome\Application\chrome.exe'; Source = 'LocalAppData' },
        @{ Path = Join-Path $env:ProgramFiles 'Google\Chrome\Application\chrome.exe'; Source = 'ProgramFiles' }
    )

    if (${env:ProgramFiles(x86)}) {
        $allowedCandidates += @{
            Path = Join-Path ${env:ProgramFiles(x86)} 'Google\Chrome\Application\chrome.exe'
            Source = 'ProgramFilesX86'
        }
    }

    $allowedPaths = @{}
    foreach ($candidate in $allowedCandidates) {
        $normalized = [IO.Path]::GetFullPath($candidate.Path)
        $allowedPaths[$normalized.ToLowerInvariant()] = $candidate
    }

    $registryCandidates = @(
        @{ Hive = [Microsoft.Win32.RegistryHive]::CurrentUser; View = [Microsoft.Win32.RegistryView]::Default; Source = 'AppPathsCurrentUser' },
        @{ Hive = [Microsoft.Win32.RegistryHive]::LocalMachine; View = [Microsoft.Win32.RegistryView]::Registry64; Source = 'AppPathsLocalMachine64' },
        @{ Hive = [Microsoft.Win32.RegistryHive]::LocalMachine; View = [Microsoft.Win32.RegistryView]::Registry32; Source = 'AppPathsLocalMachine32' }
    )

    foreach ($candidate in $registryCandidates) {
        $baseKey = $null
        $appKey = $null
        try {
            $baseKey = [Microsoft.Win32.RegistryKey]::OpenBaseKey($candidate.Hive, $candidate.View)
            $appKey = $baseKey.OpenSubKey('Software\Microsoft\Windows\CurrentVersion\App Paths\chrome.exe')
            if ($null -ne $appKey) {
                $path = [string]$appKey.GetValue($null)
                if ($path) {
                    $normalized = [IO.Path]::GetFullPath($path)
                    if (
                        $allowedPaths.ContainsKey($normalized.ToLowerInvariant()) -and
                        (Test-Path -LiteralPath $normalized -PathType Leaf)
                    ) {
                        return [pscustomobject]@{
                            Path = $normalized
                            Source = $candidate.Source
                        }
                    }
                }
            }
        }
        finally {
            if ($null -ne $appKey) { $appKey.Dispose() }
            if ($null -ne $baseKey) { $baseKey.Dispose() }
        }
    }

    foreach ($candidate in $allowedCandidates) {
        if (Test-Path -LiteralPath $candidate.Path -PathType Leaf) {
            return [pscustomobject]@{
                Path = [IO.Path]::GetFullPath($candidate.Path)
                Source = $candidate.Source
            }
        }
    }

    return $null
}

function Get-ChromeWindows {
    $windows = [System.Collections.Generic.List[object]]::new()
    $callback = [WindowProbe+EnumWindowsProc]{
        param([IntPtr]$hwnd, [IntPtr]$lParam)

        if (-not [WindowProbe]::IsWindowVisible($hwnd)) {
            return $true
        }

        $className = [Text.StringBuilder]::new(256)
        [void][WindowProbe]::GetClassName($hwnd, $className, $className.Capacity)
        if ($className.ToString() -ne 'Chrome_WidgetWin_1') {
            return $true
        }

        [uint32]$processId = 0
        [void][WindowProbe]::GetWindowThreadProcessId($hwnd, [ref]$processId)
        try {
            $process = Get-Process -Id $processId -ErrorAction Stop
            if ($process.ProcessName -ne 'chrome') {
                return $true
            }
        }
        catch {
            return $true
        }

        $windows.Add([pscustomobject]@{
            Handle = $hwnd.ToInt64()
            ProcessId = $processId
            ProcessStartTimeUtcTicks = $process.StartTime.ToUniversalTime().Ticks
            ClassName = $className.ToString()
        })
        return $true
    }

    [void][WindowProbe]::EnumWindows($callback, [IntPtr]::Zero)
    return $windows
}

function Assert-ChromeWindowIdentity {
    param([Parameter(Mandatory)][psobject]$Window)

    $hwnd = [IntPtr]::new($Window.Handle)
    if (-not [WindowProbe]::IsWindow($hwnd)) {
        throw 'The Chrome candidate window is no longer valid.'
    }

    $className = [Text.StringBuilder]::new(256)
    [void][WindowProbe]::GetClassName($hwnd, $className, $className.Capacity)
    if ($className.ToString() -ne $Window.ClassName) {
        throw 'The Chrome candidate window class changed.'
    }

    [uint32]$processId = 0
    [void][WindowProbe]::GetWindowThreadProcessId($hwnd, [ref]$processId)
    if ($processId -ne $Window.ProcessId) {
        throw 'The Chrome candidate window process changed.'
    }

    $process = Get-Process -Id $processId -ErrorAction Stop
    if (
        $process.ProcessName -ne 'chrome' -or
        $process.StartTime.ToUniversalTime().Ticks -ne $Window.ProcessStartTimeUtcTicks
    ) {
        throw 'The Chrome candidate process identity changed.'
    }
}

function Get-WindowMeasurement {
    param([Parameter(Mandatory)][psobject]$Window)

    Assert-ChromeWindowIdentity -Window $Window
    $hwnd = [IntPtr]::new($Window.Handle)
    $outer = [WindowProbe+Rect]::new()
    if (-not [WindowProbe]::GetWindowRect($hwnd, [ref]$outer)) {
        throw 'GetWindowRect failed.'
    }

    $visible = [WindowProbe+Rect]::new()
    $dwmResult = [WindowProbe]::DwmGetWindowAttribute(
        $hwnd,
        9,
        [ref]$visible,
        [Runtime.InteropServices.Marshal]::SizeOf([type][WindowProbe+Rect])
    )

    $visibleBounds = $null
    if ($dwmResult -eq 0) {
        $visibleBounds = [ordered]@{
            X = $visible.Left
            Y = $visible.Top
            Width = $visible.Right - $visible.Left
            Height = $visible.Bottom - $visible.Top
        }
    }

    return [ordered]@{
        OuterBounds = [ordered]@{
            X = $outer.Left
            Y = $outer.Top
            Width = $outer.Right - $outer.Left
            Height = $outer.Bottom - $outer.Top
        }
        VisibleBounds = $visibleBounds
        Dpi = [WindowProbe]::GetDpiForWindow($hwnd)
    }
}

function Get-PrimaryDisplay {
    $origin = [WindowProbe+Point]::new()
    $origin.X = 0
    $origin.Y = 0
    $monitor = [WindowProbe]::MonitorFromPoint($origin, 2)
    if ($monitor -eq [IntPtr]::Zero) {
        throw 'Failed to get the primary monitor.'
    }

    $info = [WindowProbe+MonitorInfo]::new()
    $info.Size = [Runtime.InteropServices.Marshal]::SizeOf([type][WindowProbe+MonitorInfo])
    if (-not [WindowProbe]::GetMonitorInfo($monitor, [ref]$info)) {
        throw 'Failed to get the primary monitor information.'
    }

    return [ordered]@{
        Handle = $monitor.ToInt64()
        MonitorArea = [ordered]@{
            X = $info.Monitor.Left
            Y = $info.Monitor.Top
            Width = $info.Monitor.Right - $info.Monitor.Left
            Height = $info.Monitor.Bottom - $info.Monitor.Top
        }
        WorkArea = [ordered]@{
            X = $info.Work.Left
            Y = $info.Work.Top
            Width = $info.Work.Right - $info.Work.Left
            Height = $info.Work.Bottom - $info.Work.Top
        }
    }
}

function Set-RequestedWindowPosition {
    param(
        [Parameter(Mandatory)][psobject]$Window,
        [Parameter(Mandatory)][System.Collections.IDictionary]$PrimaryDisplay,
        [Parameter(Mandatory)][int]$LogicalX,
        [Parameter(Mandatory)][int]$LogicalY,
        [Parameter(Mandatory)][int]$LogicalWidth,
        [Parameter(Mandatory)][int]$LogicalHeight
    )

    Assert-ChromeWindowIdentity -Window $Window
    $scale = $PrimaryDisplay.Scale
    $monitorArea = $PrimaryDisplay.MonitorArea
    $physical = [ordered]@{
        X = $monitorArea.X + [Math]::Round($LogicalX * $scale, [MidpointRounding]::AwayFromZero)
        Y = $monitorArea.Y + [Math]::Round($LogicalY * $scale, [MidpointRounding]::AwayFromZero)
        Width = [Math]::Round($LogicalWidth * $scale, [MidpointRounding]::AwayFromZero)
        Height = [Math]::Round($LogicalHeight * $scale, [MidpointRounding]::AwayFromZero)
    }

    $workArea = $PrimaryDisplay.WorkArea
    if (
        $physical.X -lt $workArea.X -or
        $physical.Y -lt $workArea.Y -or
        $physical.X + $physical.Width -gt $workArea.X + $workArea.Width -or
        $physical.Y + $physical.Height -gt $workArea.Y + $workArea.Height
    ) {
        throw 'The requested window bounds exceed the primary display work area.'
    }

    $noZOrder = 0x0004
    $noActivate = 0x0010
    $succeeded = [WindowProbe]::SetWindowPos(
        [IntPtr]::new($Window.Handle),
        [IntPtr]::Zero,
        $physical.X,
        $physical.Y,
        $physical.Width,
        $physical.Height,
        $noZOrder -bor $noActivate
    )
    if (-not $succeeded) {
        throw 'SetWindowPos failed.'
    }

    return $physical
}

function Move-WindowToPrimaryDisplay {
    param(
        [Parameter(Mandatory)][psobject]$Window,
        [Parameter(Mandatory)][System.Collections.IDictionary]$PrimaryDisplay
    )

    Assert-ChromeWindowIdentity -Window $Window
    $physicalX = $PrimaryDisplay.WorkArea.X
    $physicalY = $PrimaryDisplay.WorkArea.Y

    $noSize = 0x0001
    $noZOrder = 0x0004
    $noActivate = 0x0010
    $succeeded = [WindowProbe]::SetWindowPos(
        [IntPtr]::new($Window.Handle),
        [IntPtr]::Zero,
        $physicalX,
        $physicalY,
        0,
        0,
        $noSize -bor $noZOrder -bor $noActivate
    )
    if (-not $succeeded) {
        throw 'SetWindowPos failed while moving the window to the primary display.'
    }

    return [ordered]@{
        X = $physicalX
        Y = $physicalY
    }
}

function Wait-ForStableMeasurement {
    param(
        [Parameter(Mandatory)][psobject]$Window,
        [Parameter(Mandatory)][DateTime]$Deadline,
        [Parameter(Mandatory)][ref]$LastMeasurement
    )

    $previous = $null
    $stableCount = 0
    $measurement = $null

    do {
        Start-Sleep -Milliseconds 250
        $measurement = Get-WindowMeasurement -Window $Window
        $LastMeasurement.Value = $measurement
        $current = $measurement | ConvertTo-Json -Depth 5 -Compress
        if ($current -eq $previous) {
            $stableCount++
        }
        else {
            $stableCount = 0
            $previous = $current
        }
    } while ($stableCount -lt 2 -and [DateTime]::UtcNow -lt $Deadline)

    return [pscustomobject]@{
        Measurement = $measurement
        Stable = $stableCount -ge 2
    }
}

function Wait-ForReplacementWindow {
    param(
        [Parameter(Mandatory)][psobject]$InitialWindow,
        [Parameter(Mandatory)][hashtable]$BeforeHandles,
        [Parameter(Mandatory)][int]$WaitSeconds
    )

    $candidate = $InitialWindow
    $replacementObserved = $false
    $observedHandles = @{}
    $observedHandles[$InitialWindow.Handle] = $true
    $deadline = [DateTime]::UtcNow.AddSeconds($WaitSeconds)

    while ([DateTime]::UtcNow -lt $deadline) {
        Start-Sleep -Milliseconds 250
        $currentWindows = @(Get-ChromeWindows)
        $currentHandles = @{}
        foreach ($window in $currentWindows) {
            $currentHandles[$window.Handle] = $true
        }

        $currentCandidates = @(
            $currentWindows | Where-Object {
                -not $BeforeHandles.ContainsKey($_.Handle)
            }
        )
        foreach ($window in $currentCandidates) {
            $observedHandles[$window.Handle] = $true
            if ($window.Handle -ne $InitialWindow.Handle) {
                $candidate = $window
                $replacementObserved = $true
            }
        }
    }

    $finalCandidates = @(
        Get-ChromeWindows | Where-Object {
            -not $BeforeHandles.ContainsKey($_.Handle)
        }
    )
    $replacementCandidates = @(
        $finalCandidates | Where-Object { $_.Handle -ne $InitialWindow.Handle }
    )

    if ($replacementCandidates.Count -eq 1) {
        $candidate = $replacementCandidates[0]
        $replacementObserved = $true
    }
    elseif ($replacementCandidates.Count -gt 1) {
        throw 'Multiple replacement Chrome windows remained after the transition wait.'
    }
    elseif ($finalCandidates.Count -gt 0) {
        $candidate = $finalCandidates[-1]
    }
    else {
        throw 'No valid Chrome candidate window remained after the transition wait.'
    }

    Assert-ChromeWindowIdentity -Window $candidate

    return [pscustomobject]@{
        Handle = $candidate.Handle
        ProcessId = $candidate.ProcessId
        ProcessStartTimeUtcTicks = $candidate.ProcessStartTimeUtcTicks
        ClassName = $candidate.ClassName
        ReplacementObserved = $replacementObserved
        ObservedCandidateCount = $observedHandles.Count
    }
}

$operationStage = 'Initialization'
$partialChange = $false
$lastMeasurement = $null

try {
$chrome = Get-ChromeExecutable
if ($null -eq $chrome) {
    [pscustomobject]@{
        ChromeDetected = $false
        LaunchRequested = [bool]$Launch
        Error = 'Chrome was not found in the supported locations.'
    } | ConvertTo-Json -Depth 5
    exit 2
}

if (-not $Launch) {
    $version = [Diagnostics.FileVersionInfo]::GetVersionInfo($chrome.Path).ProductVersion
    [pscustomobject]@{
        ChromeDetected = $true
        DetectionSource = $chrome.Source
        ChromeVersion = $version
        LaunchRequested = $false
    } | ConvertTo-Json -Depth 5
    exit 0
}

if ($ApplyPosition -and $ProfileSelectionWaitSeconds -le 0) {
    throw 'ApplyPosition requires a positive ProfileSelectionWaitSeconds value.'
}

$beforeWindows = @(Get-ChromeWindows)
$beforeHandles = @{}
foreach ($window in $beforeWindows) {
    $beforeHandles[$window.Handle] = $true
}

$arguments = @(
    '--new-window',
    "--window-position=$X,$Y",
    "--window-size=$Width,$Height"
)
[void](Start-Process -FilePath $chrome.Path -ArgumentList $arguments -PassThru)

$deadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
$newWindow = $null
do {
    Start-Sleep -Milliseconds 250
    foreach ($window in @(Get-ChromeWindows)) {
        if (-not $beforeHandles.ContainsKey($window.Handle)) {
            $newWindow = $window
            break
        }
    }
} while ($null -eq $newWindow -and [DateTime]::UtcNow -lt $deadline)

if ($null -eq $newWindow) {
    [pscustomobject]@{
        ChromeDetected = $true
        DetectionSource = $chrome.Source
        LaunchRequested = $true
        NewWindowDetected = $false
        Error = 'A new Chrome top-level window was not detected before the timeout.'
    } | ConvertTo-Json -Depth 5
    exit 3
}

$candidate = [pscustomobject]@{
    Handle = $newWindow.Handle
    ProcessId = $newWindow.ProcessId
    ProcessStartTimeUtcTicks = $newWindow.ProcessStartTimeUtcTicks
    ClassName = $newWindow.ClassName
    ReplacementObserved = $false
    ObservedCandidateCount = 1
}
if ($ProfileSelectionWaitSeconds -gt 0) {
    $candidate = Wait-ForReplacementWindow `
        -InitialWindow $newWindow `
        -BeforeHandles $beforeHandles `
        -WaitSeconds $ProfileSelectionWaitSeconds
}
if ($ApplyPosition -and -not $candidate.ReplacementObserved) {
    throw 'The profile selection window was not replaced; position adjustment was not applied.'
}

$measurementDeadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
$operationStage = 'MeasuringBeforeAdjustment'
$beforeAdjustment = Wait-ForStableMeasurement `
    -Window $candidate `
    -Deadline $measurementDeadline `
    -LastMeasurement ([ref]$lastMeasurement)
if (-not $beforeAdjustment.Stable) {
    throw 'The candidate window did not become stable before position adjustment.'
}
$primaryDisplay = Get-PrimaryDisplay
$physicalRequest = $null
$moveRequest = $null
$afterMove = $null
$afterAdjustment = $null

if ($ApplyPosition) {
    $operationStage = 'MovingToPrimary'
    $moveRequest = Move-WindowToPrimaryDisplay `
        -Window $candidate `
        -PrimaryDisplay $primaryDisplay
    $partialChange = $true
    $moveDeadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $operationStage = 'MeasuringAfterMove'
    $afterMove = Wait-ForStableMeasurement `
        -Window $candidate `
        -Deadline $moveDeadline `
        -LastMeasurement ([ref]$lastMeasurement)
    if (-not $afterMove.Stable) {
        throw 'The candidate window did not become stable after moving to the primary display.'
    }
    $windowMonitor = [WindowProbe]::MonitorFromWindow(
        [IntPtr]::new($candidate.Handle),
        0
    )
    if ($windowMonitor.ToInt64() -ne $primaryDisplay.Handle) {
        throw 'The candidate window did not move to the primary monitor.'
    }
    $primaryDisplay['Dpi'] = $afterMove.Measurement.Dpi
    $primaryDisplay['Scale'] = $afterMove.Measurement.Dpi / 96.0

    $operationStage = 'ApplyingFinalBounds'
    $physicalRequest = Set-RequestedWindowPosition `
        -Window $candidate `
        -PrimaryDisplay $primaryDisplay `
        -LogicalX $X `
        -LogicalY $Y `
        -LogicalWidth $Width `
        -LogicalHeight $Height
    $adjustmentDeadline = [DateTime]::UtcNow.AddSeconds($TimeoutSeconds)
    $operationStage = 'MeasuringAfterFinalAdjustment'
    $afterAdjustment = Wait-ForStableMeasurement `
        -Window $candidate `
        -Deadline $adjustmentDeadline `
        -LastMeasurement ([ref]$lastMeasurement)
    if (-not $afterAdjustment.Stable) {
        throw 'The candidate window did not become stable after final position adjustment.'
    }

    $operationStage = 'VerifyingFinalBounds'
    $finalBounds = $afterAdjustment.Measurement.OuterBounds
    if (
        $finalBounds.X -ne $physicalRequest.X -or
        $finalBounds.Y -ne $physicalRequest.Y -or
        $finalBounds.Width -ne $physicalRequest.Width -or
        $finalBounds.Height -ne $physicalRequest.Height
    ) {
        throw 'The final window bounds do not match the requested physical bounds.'
    }
}

$result = [ordered]@{
    Succeeded = $true
    ChromeDetected = $true
    DetectionSource = $chrome.Source
    LaunchRequested = $true
    NewWindowDetected = $true
    ExistingWindowCount = $beforeWindows.Count
    WindowKind = 'UnverifiedChromeTopLevelWindow'
    WindowReplacementObserved = $candidate.ReplacementObserved
    ObservedCandidateCount = $candidate.ObservedCandidateCount
    Requested = [ordered]@{ X = $X; Y = $Y; Width = $Width; Height = $Height }
    PrimaryDisplay = $primaryDisplay
    MeasurementStableBeforeAdjustment = $beforeAdjustment.Stable
    ObservedBeforeAdjustment = $beforeAdjustment.Measurement
    PositionAdjustmentApplied = [bool]$ApplyPosition
}

if ($ApplyPosition) {
    $result['MoveRequest'] = $moveRequest
    $result['MeasurementStableAfterMove'] = $afterMove.Stable
    $result['ObservedAfterMove'] = $afterMove.Measurement
    $result['PhysicalRequest'] = $physicalRequest
    $result['MeasurementStableAfterAdjustment'] = $afterAdjustment.Stable
    $result['ObservedAfterAdjustment'] = $afterAdjustment.Measurement
}

[pscustomobject]$result | ConvertTo-Json -Depth 6
}
catch {
    [pscustomobject]@{
        Succeeded = $false
        OperationStage = $operationStage
        PartialChangeApplied = $partialChange
        LastMeasurement = $lastMeasurement
        Error = $_.Exception.Message
    } | ConvertTo-Json -Depth 6
    exit 4
}
