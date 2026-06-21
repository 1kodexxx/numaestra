import { useEffect, useState } from 'react'
import { categoryApi } from '@entities/category'
import type { Category } from '@entities/category'

interface State {
  categories: Category[]
  loading: boolean
  error: string | null
}

export function useCatalog() {
  const [state, setState] = useState<State>({ categories: [], loading: true, error: null })

  useEffect(() => {
    let cancelled = false
    categoryApi
      .list()
      .then((categories) => {
        if (!cancelled) setState({ categories, loading: false, error: null })
      })
      .catch((err: Error) => {
        if (!cancelled) setState({ categories: [], loading: false, error: err.message })
      })
    return () => { cancelled = true }
  }, [])

  return state
}
