import {cleanup, render, screen, waitFor} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest'
import App from './App'
import {
  CancelLaunch,
  GetCurrentLaunchState,
  StartLaunch,
} from '../wailsjs/go/wailsadapter/LaunchOperationController'
import {EventsOn} from '../wailsjs/runtime/runtime'
import {wailsadapter} from '../wailsjs/go/models'

const runtimeMock = vi.hoisted(() => ({
  listener: undefined as ((state: LaunchState) => void) | undefined,
  unsubscribe: vi.fn(),
}))

vi.mock('../wailsjs/go/wailsadapter/AppController', () => ({
  GetApplicationInfo: vi.fn().mockResolvedValue({name: 'Browser Launcher', version: 'dev'}),
}))

vi.mock('../wailsjs/go/wailsadapter/LaunchOperationController', () => ({
  CancelLaunch: vi.fn(),
  GetCurrentLaunchState: vi.fn(),
  StartLaunch: vi.fn(),
}))

vi.mock('../wailsjs/runtime/runtime', () => ({
  EventsOn: vi.fn((_event: string, listener: (state: LaunchState) => void) => {
    runtimeMock.listener = listener
    return runtimeMock.unsubscribe
  }),
}))

beforeEach(() => {
  vi.mocked(GetCurrentLaunchState).mockResolvedValue(
    new wailsadapter.GetLaunchStateResultDTO({found: false}),
  )
  vi.mocked(CancelLaunch).mockResolvedValue(
    new wailsadapter.CancelLaunchResultDTO({status: 'accepted'}),
  )
})

afterEach(() => {
  cleanup()
  vi.clearAllMocks()
  runtimeMock.listener = undefined
})

describe('App', () => {
  it('入力値をGoへ渡して起動し、状態イベントを表示する', async () => {
    vi.mocked(StartLaunch).mockResolvedValue(startedResult())
    const user = userEvent.setup()
    render(<App />)

    await enterValidValues(user)
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))

    await waitFor(() => {
      expect(StartLaunch).toHaveBeenCalledWith({
        width: '1280', height: '720', x: '0', y: '100',
      })
    })
    runtimeMock.listener?.(launchState('running', 'Chromeの起動とウィンドウ配置を実行しています。'))
    expect(await screen.findByText('Chromeの起動とウィンドウ配置を実行しています。')).toBeTruthy()
    expect(screen.getByText('Operation ID: operation-1')).toBeTruthy()
  })

  it('無効な入力では項目別エラーを表示し、操作を開始済みとして扱わない', async () => {
    vi.mocked(StartLaunch).mockResolvedValue(new wailsadapter.StartLaunchResultDTO({
      started: false,
      operationId: '',
      state: '',
      validation: validationResult(false, [
        {field: 'width', message: '値を入力してください。'},
        {field: 'height', message: '1以上の整数を入力してください。'},
      ]),
    }))
    const user = userEvent.setup()
    render(<App />)

    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))

    expect(await screen.findByText('値を入力してください。')).toBeTruthy()
    expect(screen.getByText('1以上の整数を入力してください。')).toBeTruthy()
    expect(screen.queryByText(/Operation ID:/)).toBeNull()
    expect(screen.getByLabelText('幅').getAttribute('aria-invalid')).toBe('true')
  })

  it('実行中は重複起動を防止し、取消要求を送信する', async () => {
    vi.mocked(StartLaunch).mockResolvedValue(startedResult())
    vi.mocked(CancelLaunch).mockResolvedValue(new wailsadapter.CancelLaunchResultDTO({
      status: 'accepted',
      state: launchState('cancelling', 'Chromeの起動処理を取り消しています。'),
    }))
    const user = userEvent.setup()
    render(<App />)
    await enterValidValues(user)
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))

    const launchButton = await screen.findByRole('button', {name: 'Chromeを起動'})
    expect((launchButton as HTMLButtonElement).disabled).toBe(true)
    await user.click(screen.getByRole('button', {name: '起動を取り消す'}))

    expect(CancelLaunch).toHaveBeenCalledWith('operation-1')
    expect(await screen.findByText('Chromeの起動処理を取り消しています。')).toBeTruthy()
  })

  it('取消要求中に受信した完了イベントを遅い応答で上書きしない', async () => {
    vi.mocked(StartLaunch).mockResolvedValue(startedResult())
    let resolveCancel!: (value: wailsadapter.CancelLaunchResultDTO) => void
    vi.mocked(CancelLaunch).mockReturnValue(new Promise((resolve) => {
      resolveCancel = resolve
    }))
    const user = userEvent.setup()
    render(<App />)
    await enterValidValues(user)
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))
    await user.click(screen.getByRole('button', {name: '起動を取り消す'}))

    runtimeMock.listener?.(launchState('cancelled', 'Chromeの起動処理を取り消しました。'))
    resolveCancel(new wailsadapter.CancelLaunchResultDTO({
      status: 'accepted',
      state: launchState('cancelling', 'Chromeの起動処理を取り消しています。'),
    }))

    expect(await screen.findByText('Chromeの起動処理を取り消しました。')).toBeTruthy()
    expect(screen.queryByText('Chromeの起動処理を取り消しています。')).toBeNull()
  })

  it('取消後の遅い再取得応答で新しい操作を上書きしない', async () => {
    vi.mocked(StartLaunch)
      .mockResolvedValueOnce(startedResult())
      .mockResolvedValueOnce(new wailsadapter.StartLaunchResultDTO({
        started: true,
        operationId: 'operation-2',
        state: 'starting',
        validation: validationResult(true),
      }))
    let resolveRefresh!: (value: wailsadapter.GetLaunchStateResultDTO) => void
    vi.mocked(CancelLaunch).mockResolvedValue(
      new wailsadapter.CancelLaunchResultDTO({status: 'already_finished'}),
    )
    vi.mocked(GetCurrentLaunchState)
      .mockResolvedValueOnce(new wailsadapter.GetLaunchStateResultDTO({found: false}))
      .mockReturnValueOnce(new Promise((resolve) => {
        resolveRefresh = resolve
      }))
    const user = userEvent.setup()
    render(<App />)
    await enterValidValues(user)
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))
    await user.click(screen.getByRole('button', {name: '起動を取り消す'}))

    runtimeMock.listener?.({
      operationId: 'operation-1', state: 'completed', errorCode: '', message: '操作1完了',
    })
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))
    runtimeMock.listener?.({
      operationId: 'operation-2', state: 'running', errorCode: '', message: '操作2実行中',
    })
    resolveRefresh(currentStateResult('completed', '古い操作1完了'))

    expect(await screen.findByText('操作2実行中')).toBeTruthy()
    expect(screen.queryByText('古い操作1完了')).toBeNull()
    expect(screen.getByText('Operation ID: operation-2')).toBeTruthy()
  })

  it('終了後も入力値を保持し、再実行できる', async () => {
    vi.mocked(StartLaunch).mockResolvedValue(startedResult())
    const user = userEvent.setup()
    render(<App />)
    await enterValidValues(user)
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))

    runtimeMock.listener?.(launchState('completed', 'Chromeウィンドウを起動して配置しました。'))

    expect(await screen.findByText('Chromeウィンドウを起動して配置しました。')).toBeTruthy()
    expect((screen.getByLabelText('幅') as HTMLInputElement).value).toBe('1280')
    expect((screen.getByRole('button', {name: 'Chromeを起動'}) as HTMLButtonElement).disabled).toBe(false)
  })

  it('起動失敗後も入力値を保持して再実行できる', async () => {
    vi.mocked(StartLaunch)
      .mockResolvedValueOnce(startedResult())
      .mockResolvedValueOnce(new wailsadapter.StartLaunchResultDTO({
        started: true,
        operationId: 'operation-2',
        state: 'starting',
        validation: validationResult(true),
      }))
    const user = userEvent.setup()
    render(<App />)
    await enterValidValues(user)
    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))

    runtimeMock.listener?.({
      operationId: 'operation-1',
      state: 'failed',
      errorCode: 'launch_failed',
      message: 'Chromeウィンドウを起動または配置できませんでした。画面とChromeの状態を確認して再実行してください。',
    })

    expect((await screen.findByRole('alert')).textContent).toContain(
      'Chromeウィンドウを起動または配置できませんでした。画面とChromeの状態を確認して再実行してください。',
    )
    expect((screen.getByLabelText('幅') as HTMLInputElement).value).toBe('1280')
    expect((screen.getByLabelText('高さ') as HTMLInputElement).value).toBe('720')
    expect((screen.getByLabelText('X座標') as HTMLInputElement).value).toBe('0')
    expect((screen.getByLabelText('Y座標') as HTMLInputElement).value).toBe('100')

    const launchButton = screen.getByRole('button', {name: 'Chromeを起動'})
    expect((launchButton as HTMLButtonElement).disabled).toBe(false)
    await user.click(launchButton)

    await waitFor(() => {
      expect(StartLaunch).toHaveBeenCalledTimes(2)
    })
    expect(StartLaunch).toHaveBeenLastCalledWith({
      width: '1280', height: '720', x: '0', y: '100',
    })
  })

  it('再マウント時にApplicationの最新状態を復元する', async () => {
    vi.mocked(GetCurrentLaunchState).mockResolvedValue(new wailsadapter.GetLaunchStateResultDTO({
      found: true,
      state: launchState('running', 'Chromeの起動とウィンドウ配置を実行しています.'),
    }))

    const view = render(<App />)
    expect(await screen.findByText('Operation ID: operation-1')).toBeTruthy()
    view.unmount()
    render(<App />)

    expect(await screen.findByText('Operation ID: operation-1')).toBeTruthy()
    expect(EventsOn).toHaveBeenCalledTimes(2)
    expect(runtimeMock.unsubscribe).toHaveBeenCalledTimes(1)
  })

  it('状態取得中に受信した新しいイベントを遅い応答で上書きしない', async () => {
    let resolveState!: (value: wailsadapter.GetLaunchStateResultDTO) => void
    vi.mocked(GetCurrentLaunchState).mockReturnValue(
      new Promise((resolve) => {
        resolveState = resolve
      }),
    )
    render(<App />)

    await waitFor(() => expect(runtimeMock.listener).toBeDefined())
    runtimeMock.listener?.(launchState('completed', '新しい完了状態'))
    resolveState(currentStateResult('running', '古い実行中状態'))

    expect(await screen.findByText('新しい完了状態')).toBeTruthy()
    expect(screen.queryByText('古い実行中状態')).toBeNull()
  })

  it('開始失敗では安全な再試行メッセージを表示する', async () => {
    vi.mocked(StartLaunch).mockRejectedValue(
      String.raw`open C:\Users\someone\secret: access denied`,
    )
    const user = userEvent.setup()
    render(<App />)
    await enterValidValues(user)

    await user.click(screen.getByRole('button', {name: 'Chromeを起動'}))

    expect(
      await screen.findByText('Chromeの起動処理を開始できませんでした。もう一度お試しください。'),
    ).toBeTruthy()
  })
})

type LaunchState = {
  operationId: string
  state: string
  errorCode: string
  message: string
}

function startedResult() {
  return new wailsadapter.StartLaunchResultDTO({
    started: true,
    operationId: 'operation-1',
    state: 'starting',
    validation: validationResult(true),
  })
}

function validationResult(
  valid: boolean,
  errors: Array<{field: string; message: string}> = [],
  generalError = '',
) {
  return new wailsadapter.ValidationResultDTO({valid, errors, generalError})
}

function launchState(state: string, message: string): LaunchState {
  return {operationId: 'operation-1', state, errorCode: '', message}
}

function currentStateResult(state: string, message: string) {
  return new wailsadapter.GetLaunchStateResultDTO({
    found: true,
    state: launchState(state, message),
  })
}

async function enterValidValues(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('幅'), '1280')
  await user.type(screen.getByLabelText('高さ'), '720')
  await user.type(screen.getByLabelText('X座標'), '0')
  await user.type(screen.getByLabelText('Y座標'), '100')
}
