import React, {useState, useEffect} from 'react'
import { uploadDocument, chatRequest, getDocuments } from './api'

type Doc = {
  id: string
  filename?: string
  status?: string
  chunk_count?: number
}

export default function App(): JSX.Element {
  const [file, setFile] = useState<File | null>(null)
  const [uploadStatus, setUploadStatus] = useState<string>('')
  const [question, setQuestion] = useState<string>('')
  const [answer, setAnswer] = useState<string>('')
  const [docs, setDocs] = useState<Doc[]>([])

  async function handleUpload(e: React.FormEvent) {
    e.preventDefault()
    if (!file) return
    setUploadStatus('Uploading...')
    try {
      const res = await uploadDocument(file)
      setUploadStatus('Uploaded: ' + (res?.id || 'ok'))
      fetchDocs()
    } catch (err: any) {
      setUploadStatus('Upload failed: ' + (err?.message || String(err)))
    }
  }

  async function handleChat(e: React.FormEvent) {
    e.preventDefault()
    if (!question) return
    setAnswer('Thinking...')
    try {
      const res = await chatRequest({question, stream: false})
      setAnswer(res.answer || JSON.stringify(res))
    } catch (err: any) {
      setAnswer('Chat failed: ' + (err?.message || String(err)))
    }
  }

  async function fetchDocs() {
    try {
      const res = await getDocuments()
      if (Array.isArray(res)) setDocs(res)
      else if (res && res.data) setDocs(res.data)
      else setDocs([])
    } catch (err) {
      console.error('fetchDocs', err)
    }
  }

  useEffect(() => {
    fetchDocs()
  }, [])

  return (
    <div className="container">
      <h1>RAG Chatbot — Frontend</h1>

      <section>
        <h2>Upload Document</h2>
        <form onSubmit={handleUpload}>
          <input type="file" onChange={e => setFile(e.target.files?.[0] || null)} />
          <button type="submit">Upload</button>
        </form>
        <p>{uploadStatus}</p>
      </section>

      <section>
        <h2>Ask a question</h2>
        <form onSubmit={handleChat}>
          <input type="text" value={question} onChange={e => setQuestion(e.target.value)} placeholder="Ask something..." />
          <button type="submit">Send</button>
        </form>
        <pre className="answer">{answer}</pre>
      </section>

      <section>
        <h2>Documents</h2>
        <button onClick={fetchDocs}>Refresh</button>
        <table style={{width:'100%', marginTop:8, borderCollapse:'collapse'}}>
          <thead>
            <tr style={{textAlign:'left'}}>
              <th>Filename</th>
              <th>Status</th>
              <th>Chunks</th>
            </tr>
          </thead>
          <tbody>
            {docs.map(d => (
              <tr key={d.id} style={{borderTop:'1px solid #eee'}}>
                <td>{d.filename}</td>
                <td>{d.status}</td>
                <td>{d.chunk_count || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  )
}
