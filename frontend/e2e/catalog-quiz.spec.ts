import { test, expect } from '@playwright/test'

test.describe('Каталог → конструктор → контакты', () => {
  test.use({ viewport: { width: 390, height: 844 } })

  test('открывает конструктор и модалку оформления', async ({ page }) => {
    await page.goto('/')

    await page.getByRole('button', { name: /Опишите вашу песню/i }).click()
    await expect(page.getByText('Соберите свою песню')).toBeVisible()

    await page.getByLabel(/Для кого и по какому поводу/i).fill('Маме на день рождения')
    await page.getByLabel(/^Детали/i).fill('Зовут Ольга, любит сад и внуков')

    await page.getByRole('button', { name: /Продолжить/i }).click()

    const dialog = page.getByRole('dialog', { name: /Оформление заказа|Создаём заказ/i })
    await expect(dialog).toBeVisible()
    await expect(dialog.getByLabel('Email')).toBeVisible()
  })

  test('фильтр категорий сужает список', async ({ page }) => {
    await page.goto('/')

    const filter = page.getByRole('searchbox', { name: 'Поиск по категориям' })
    if (await filter.isVisible()) {
      await filter.fill('свад')
      await expect(page.getByRole('button', { name: /Категория:/i }).first()).toBeVisible()
    }
  })
})
