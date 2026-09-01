import { useState } from 'react'
import { Upload } from 'antd'
import { InboxOutlined } from '@ant-design/icons'
import { toast } from '../../utils/feedback'

type Props = {
  accept: string
  hint: string
  title?: string
  loading?: boolean
  fileName?: string
  onUpload: (file: File) => Promise<void>
}

/** 拖拽上传区（向导素材步骤） */
export function MaterialDropzone({
  accept,
  hint,
  title = '拖拽文件到此处，或点击上传',
  loading,
  fileName,
  onUpload,
}: Props) {
  const [dragOver, setDragOver] = useState(false)

  return (
    <div
      className={`wz-dropzone-wrap${dragOver ? ' is-dragover' : ''}${fileName ? ' has-file' : ''}`}
      onDragOver={(e) => { e.preventDefault(); setDragOver(true) }}
      onDragLeave={() => setDragOver(false)}
      onDrop={() => setDragOver(false)}
    >
      <Upload.Dragger
        className="wz-dropzone"
        accept={accept}
        showUploadList={false}
        disabled={loading}
        customRequest={async ({ file, onSuccess, onError }) => {
          try {
            await onUpload(file as File)
            onSuccess?.({})
          } catch (e) {
            onError?.(e as Error)
          }
        }}
        beforeUpload={(file) => {
          const maxMb = 500
          if (file.size > maxMb * 1024 * 1024) {
            toast.fail(`文件不能超过 ${maxMb}MB`)
            return Upload.LIST_IGNORE
          }
          return true
        }}
      >
        <p className="wz-dropzone-icon"><InboxOutlined /></p>
        <p className="wz-dropzone-title">{fileName || title}</p>
        <p className="wz-dropzone-hint">{hint}</p>
        {loading && <p className="wz-dropzone-loading">上传处理中…</p>}
      </Upload.Dragger>
    </div>
  )
}
