import {FormEvent, useEffect, useRef, useState} from 'react'
import {GetApplicationInfo} from '../wailsjs/go/wailsadapter/AppController'
import {
  CancelLaunch,
  GetCurrentLaunchState,
  StartLaunch,
} from '../wailsjs/go/wailsadapter/LaunchOperationController'
import {EventsOn} from '../wailsjs/runtime/runtime'

type ApplicationInfo = {
  name: string
  version: string
}

type FieldName = 'width' | 'height' | 'x' | 'y'
type FormValues = Record<FieldName, string>
type FieldErrors = Partial<Record<FieldName, string>>

type LaunchState = {
  operationId: string
  state: string
  errorCode?: string
  message: string
}

const launchStateChangedEvent = 'launcher:state-changed'
const activeStates = new Set(['starting', 'running', 'cancelling'])

const fields: Array<{
  name: FieldName
  label: string
  hint: string
}> = [
  {name: 'width', label: '幅', hint: '1以上の整数'},
  {name: 'height', label: '高さ', hint: '1以上の整数'},
  {name: 'x', label: 'X座標', hint: '0以上の整数'},
  {name: 'y', label: 'Y座標', hint: '0以上の整数'},
]

const initialValues: FormValues = {
  width: '',
  height: '',
  x: '',
  y: '',
}

function App() {
  const [applicationInfo, setApplicationInfo] = useState<ApplicationInfo | null>(null)
  const [applicationInfoError, setApplicationInfoError] = useState('')
  const [values, setValues] = useState<FormValues>(initialValues)
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({})
  const [launchMessage, setLaunchMessage] = useState('')
  const [launchState, setLaunchState] = useState<LaunchState | null>(null)
  const [isStarting, setIsStarting] = useState(false)
  const [isCancelling, setIsCancelling] = useState(false)
  const launchStateRef = useRef<LaunchState | null>(null)
  const eventVersionRef = useRef(0)
  const acceptingNewOperationRef = useRef(false)

  const applyLaunchState = (next: LaunchState) => {
    launchStateRef.current = next
    setLaunchState(next)
    setLaunchMessage(next.message)
  }

  useEffect(() => {
    let active = true

    GetApplicationInfo()
      .then((info) => {
        if (active) {
          setApplicationInfo(info)
        }
      })
      .catch(() => {
        if (active) {
          setApplicationInfoError('アプリケーション情報を取得できませんでした。')
        }
      })

    const unsubscribe = EventsOn(launchStateChangedEvent, (event: LaunchState) => {
      if (!active) {
        return
      }
      const currentID = launchStateRef.current?.operationId
      if (currentID && currentID !== event.operationId && !acceptingNewOperationRef.current) {
        return
      }
      eventVersionRef.current += 1
      applyLaunchState(event)
    })

    const versionBeforeRequest = eventVersionRef.current
    GetCurrentLaunchState()
      .then((result) => {
        if (
          active &&
          result.found &&
          result.state &&
          eventVersionRef.current === versionBeforeRequest
        ) {
          applyLaunchState(result.state)
        }
      })
      .catch(() => {
        if (active && eventVersionRef.current === versionBeforeRequest) {
          setLaunchMessage('Chromeの起動状態を取得できませんでした。もう一度お試しください。')
        }
      })

    return () => {
      active = false
      unsubscribe()
    }
  }, [])

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setIsStarting(true)
    setFieldErrors({})
    setLaunchMessage('')
    acceptingNewOperationRef.current = true
    const versionBeforeRequest = eventVersionRef.current

    try {
      const result = await StartLaunch(values)
      if (!result.started) {
        const errors = result.validation.errors.reduce<FieldErrors>((accumulator, error) => {
          if (fields.some(({name}) => name === error.field)) {
            accumulator[error.field as FieldName] = error.message
          }
          return accumulator
        }, {})
        setFieldErrors(errors)
        setLaunchMessage(
          result.validation.generalError || '入力内容を確認してください。',
        )
        return
      }

      if (eventVersionRef.current === versionBeforeRequest) {
        applyLaunchState({
          operationId: result.operationId,
          state: result.state,
          errorCode: '',
          message: 'Chromeの起動処理を開始しています。',
        })
      }
    } catch {
      setLaunchMessage('Chromeの起動処理を開始できませんでした。もう一度お試しください。')
    } finally {
      acceptingNewOperationRef.current = false
      setIsStarting(false)
    }
  }

  const handleCancel = async () => {
    if (!launchState) {
      return
    }
    setIsCancelling(true)
    const versionBeforeRequest = eventVersionRef.current
    try {
      const result = await CancelLaunch(launchState.operationId)
      if (result.state && eventVersionRef.current === versionBeforeRequest) {
        applyLaunchState(result.state)
      }
      if (result.status === 'already_finished') {
        const versionBeforeRefresh = eventVersionRef.current
        const operationIDBeforeRefresh = launchStateRef.current?.operationId
        const latest = await GetCurrentLaunchState()
        if (
          latest.found &&
          latest.state &&
          eventVersionRef.current === versionBeforeRefresh &&
          launchStateRef.current?.operationId === operationIDBeforeRefresh
        ) {
          applyLaunchState(latest.state)
        }
      }
    } catch {
      setLaunchMessage('取消要求を送信できませんでした。現在の状態を確認して、もう一度お試しください。')
    } finally {
      setIsCancelling(false)
    }
  }

  const operationActive = launchState ? activeStates.has(launchState.state) : false

  return (
    <main>
      <p className="eyebrow">Browser Launcher</p>
      <h1>Chromeの起動条件を入力</h1>
      <p className="description">
        Chromeウィンドウの位置とサイズを論理ピクセルで指定して起動します。
      </p>

      <section aria-labelledby="launch-condition-heading">
        <h2 id="launch-condition-heading">起動条件</h2>
        <form onSubmit={handleSubmit} noValidate>
          <div className="form-grid">
            {fields.map(({name, label, hint}) => {
              const errorID = `${name}-error`
              const hintID = `${name}-hint`
              const describedBy = fieldErrors[name] ? `${hintID} ${errorID}` : hintID

              return (
                <div className="form-field" key={name}>
                  <label htmlFor={name}>{label}</label>
                  <span className="field-hint" id={hintID}>
                    {hint}
                  </span>
                  <input
                    aria-describedby={describedBy}
                    aria-invalid={fieldErrors[name] ? 'true' : 'false'}
                    disabled={operationActive}
                    id={name}
                    inputMode="numeric"
                    name={name}
                    onChange={(event) => {
                      setValues((current) => ({...current, [name]: event.target.value}))
                    }}
                    type="text"
                    value={values[name]}
                  />
                  {fieldErrors[name] && (
                    <span className="field-error" id={errorID}>
                      {fieldErrors[name]}
                    </span>
                  )}
                </div>
              )
            })}
          </div>

          <div className="action-row">
            <button disabled={isStarting || operationActive} type="submit">
              {isStarting ? '開始中です。' : 'Chromeを起動'}
            </button>
            {operationActive && (
              <button
                className="secondary-button"
                disabled={isCancelling || launchState?.state === 'cancelling'}
                onClick={handleCancel}
                type="button"
              >
                {isCancelling || launchState?.state === 'cancelling'
                  ? '取消中です。'
                  : '起動を取り消す'}
              </button>
            )}
          </div>
        </form>

        {launchMessage && (
          <div
            className="launch-result"
            role={launchState?.state === 'failed' ? 'alert' : 'status'}
          >
            <p>{launchMessage}</p>
            {launchState && <p className="operation-id">Operation ID: {launchState.operationId}</p>}
          </div>
        )}
      </section>

      <section aria-labelledby="application-info-heading">
        <h2 id="application-info-heading">アプリケーション情報</h2>
        {applicationInfoError && <p role="alert">{applicationInfoError}</p>}
        {!applicationInfo && !applicationInfoError && <p>読み込み中です。</p>}
        {applicationInfo && (
          <dl>
            <dt>名前</dt>
            <dd>{applicationInfo.name}</dd>
            <dt>バージョン</dt>
            <dd>{applicationInfo.version}</dd>
          </dl>
        )}
      </section>
    </main>
  )
}

export default App
