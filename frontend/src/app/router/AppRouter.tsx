import { Routes, Route, Navigate } from 'react-router-dom'
import { CatalogPage } from '@pages/catalog'
import { QuizPage } from '@pages/quiz'
import { StatusPage } from '@pages/status'

export function AppRouter() {
  return (
    <Routes>
      <Route path="/" element={<CatalogPage />} />
      <Route path="/quiz/:categoryId" element={<QuizPage />} />
      <Route path="/status" element={<StatusPage />} />
      {/* После возврата с Robokassa — сразу на статус */}
      <Route path="/order/success" element={<Navigate to="/status" replace />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
