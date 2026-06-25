import { useCallback, useEffect, useState } from 'react'
import { categoryApi } from '@entities/category'
import type { Category } from '@entities/category'

interface State {
  categories: Category[]
  loading: boolean
  error: string | null
}

export function useCatalog() {
  const [reloadKey, setReloadKey] = useState(0)
  const [state, setState] = useState<State>({ categories: [], loading: true, error: null })

  const reload = useCallback(() => setReloadKey((k) => k + 1), [])

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    categoryApi
      .list()
      .then((categories) => {
        if (!cancelled) setState({ categories, loading: false, error: null })
      })
      .catch((err: Error) => {
        if (!cancelled) setState({ categories: [], loading: false, error: err.message })
      })
    return () => { cancelled = true }
  }, [reloadKey])

  return { ...state, reload }
}
