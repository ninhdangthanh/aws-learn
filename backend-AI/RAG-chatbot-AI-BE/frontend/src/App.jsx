import React, {useState} from 'react'
import { uploadDocument, chatRequest } from './api'

export default function App() {
  const [file, setFile] = useState(null)
  const [uploadStatus, setUploadStatus] = useState('')
  const [question, setQuestion] = useState('')
  const [answer, setAnswer] = useState('')

  async function handleUpload(e) {
    e.preventDefault()
    if (!file) return
    setUploadStatus('Uploading...')
    try {
      const res = await uploadDocument(file)
      setUploadStatus('Uploaded: ' + (res?.id || 'ok'))
    } catch (err) {
      setUploadStatus('Upload failed: ' + err.message)
    }
  }

  async function handleChat(e) {
    e.preventDefault()
    if (!question) return
    setAnswer('Thinking...')
    try {
      const res = await chatRequest({question, stream: false})
      setAnswer(res.answer || JSON.stringify(res))
    } catch (err) {
      setAnswer('Chat failed: ' + err.message)
    }
  }

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
    </div>
  )
}
