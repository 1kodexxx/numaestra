import { Link } from 'react-router-dom'
import styles from './Navbar.module.css'

export function Navbar() {
  return (
    <nav className={styles.nav}>
      <Link to="/" className={styles.logo}>🎵 Numaestra</Link>
      <div className={styles.spacer} />
      <Link to="/" className={styles.link}>Каталог</Link>
      <Link to="/status" className={styles.link}>Мой заказ</Link>
    </nav>
  )
}
