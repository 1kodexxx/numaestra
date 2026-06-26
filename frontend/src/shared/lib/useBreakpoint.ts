import { useEffect, useState } from 'react'

/** Адаптивные брейкпоинты витрины (синхронны с CSS media). */
export function useBreakpoint() {
  const [vp, setVp] = useState(() => ({
    w: typeof window !== 'undefined' ? window.innerWidth : 1200,
    h: typeof window !== 'undefined' ? window.innerHeight : 800,
  }))

  useEffect(() => {
    const fn = () => setVp({ w: window.innerWidth, h: window.innerHeight })
    window.addEventListener('resize', fn)
    return () => window.removeEventListener('resize', fn)
  }, [])

  return {
    width: vp.w,
    height: vp.h,
    isMobile: vp.w < 640,
    isTablet: vp.w >= 640 && vp.w < 1024,
    isDesktop: vp.w >= 1024,
    isShort: vp.h < 720,
  }
}
