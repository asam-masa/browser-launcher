import {useCallback, useEffect, useRef, useState} from 'react'
import {
  CancelOperation,
  GetOperationState,
  StartOperation,
} from '../wailsjs/go/main/App'
import {EventsOn} from '../wailsjs/runtime/runtime'

type OperationState = {
  operationId: string
  state: string
  errorCode?: string
  message?: string
}

const stateEvent = 'operation:state-changed'

function App() {
  const [currentOperationId, setCurrentOperationId] = useState('')
  const [currentState, setCurrentState] = useState<OperationState | null>(null)
  const [events, setEvents] = useState<OperationState[]>([])
  const [listenerEnabled, setListenerEnabled] = useState(true)
  const [requestError, setRequestError] = useState('')
  const latestStates = useRef(new Map<string, OperationState>())

  useEffect(() => {
    if (!listenerEnabled) {
      return
    }

    return EventsOn(stateEvent, (received: OperationState) => {
      latestStates.current.set(received.operationId, received)
      setEvents((current) => [...current, received])
      setCurrentState((current) => {
        if (!current || current.operationId === received.operationId) {
          return received
        }
        return current
      })
    })
  }, [listenerEnabled])

  const start = useCallback(async (mode: string, durationMilliseconds = 1500) => {
    setRequestError('')
    try {
      const result = await StartOperation({mode, durationMilliseconds})
      setCurrentOperationId(result.operationId)
      setCurrentState(
        latestStates.current.get(result.operationId) ?? {
          operationId: result.operationId,
          state: result.state,
        },
      )
    } catch (error) {
      setRequestError(String(error))
    }
  }, [])

  const cancel = useCallback(async () => {
    if (!currentOperationId) {
      return
    }
    await CancelOperation(currentOperationId)
  }, [currentOperationId])

  const refresh = useCallback(async () => {
    if (!currentOperationId) {
      return
    }
    const result = await GetOperationState(currentOperationId)
    if (result.found && result.state) {
      setCurrentState(result.state)
    }
  }, [currentOperationId])

  return (
    <main>
      <h1>Wails Communication Spike</h1>
      <p>
        Goメソッドの呼び出し、状態イベント、取消、タイムアウト、失敗、
        状態の再取得を確認します。
      </p>

      <section aria-labelledby="actions-heading">
        <h2 id="actions-heading">検証操作</h2>
        <div className="actions">
          <button onClick={() => start('complete')}>正常完了を開始</button>
          <button onClick={() => start('timeout')}>タイムアウトを開始</button>
          <button onClick={() => start('fail')}>非同期失敗を開始</button>
          <button onClick={() => start('invalid')}>開始前エラーを発生</button>
          <button onClick={cancel} disabled={!currentOperationId}>
            現在の操作を取り消す
          </button>
          <button onClick={refresh} disabled={!currentOperationId}>
            最新状態を取得
          </button>
          <button onClick={() => setListenerEnabled((enabled) => !enabled)}>
            状態リスナーを{listenerEnabled ? '解除' : '登録'}
          </button>
        </div>
      </section>

      <section aria-labelledby="state-heading">
        <h2 id="state-heading">現在の状態</h2>
        <dl>
          <dt>Operation ID</dt>
          <dd>{currentOperationId || 'なし'}</dd>
          <dt>State</dt>
          <dd>{currentState?.state || 'なし'}</dd>
          <dt>Error Code</dt>
          <dd>{currentState?.errorCode || 'なし'}</dd>
          <dt>Message</dt>
          <dd>{currentState?.message || 'なし'}</dd>
        </dl>
        {requestError && (
          <p role="alert" className="error">
            Promise rejection: {requestError}
          </p>
        )}
      </section>

      <section aria-labelledby="events-heading">
        <h2 id="events-heading">受信したイベント</h2>
        {events.length === 0 ? (
          <p>イベントはありません。</p>
        ) : (
          <ol>
            {events.map((event, index) => (
              <li key={`${event.operationId}-${event.state}-${index}`}>
                {event.operationId}: {event.state}
                {event.errorCode ? ` (${event.errorCode})` : ''}
              </li>
            ))}
          </ol>
        )}
      </section>
    </main>
  )
}

export default App
