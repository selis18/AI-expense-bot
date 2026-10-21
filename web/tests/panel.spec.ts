import { expect, test } from '@playwright/test';

test('create, filter, edit and delete a family expense', async ({ page, request }) => {
  const users = await (await request.get('/api/users')).json();
  expect(users.items.length, 'Seed at least one user in the test database').toBeGreaterThan(0);
  const comment = `Browser test ${Date.now()}`;
  let id: number | undefined;
  try {
    await page.goto('/');
    await page.getByRole('button', { name: 'Добавить расход' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Сумма, ₽').fill('350,50');
    await dialog.getByLabel('Дата', { exact: true }).fill('2026-10-21');
    await dialog.getByLabel('Комментарий', { exact: false }).fill(comment);
    const created = page.waitForResponse(response => response.url().endsWith('/api/expenses') && response.request().method() === 'POST');
    await dialog.getByRole('button', { name: 'Сохранить' }).click();
    const response = await created;
    expect(response.status()).toBe(201);
    id = (await response.json()).id;
    await expect(page.getByText(comment, { exact: true })).toBeVisible();
    await page.getByRole('button', { name: `Изменить расход ${id}`, exact: true }).click();
    await dialog.getByLabel('Сумма, ₽').fill('450.75');
    await dialog.getByRole('button', { name: 'Сохранить' }).click();
    await expect(page.getByRole('row').filter({ hasText: comment })).toContainText('450,75');
    await page.locator('.filters').getByLabel('Категория').selectOption('транспорт');
    await page.getByRole('button', { name: 'Показать', exact: true }).click();
    await expect(page.getByText(comment, { exact: true })).toHaveCount(0);
    await page.getByRole('button', { name: 'Сбросить', exact: true }).click();
    await expect(page.getByText(comment, { exact: true })).toBeVisible();
    await page.screenshot({ path: 'test-results/panel-desktop.png', fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: 'test-results/panel-mobile.png', fullPage: true });
    await page.getByRole('button', { name: `Удалить расход ${id}`, exact: true }).click();
    await dialog.getByRole('button', { name: 'Отмена' }).click();
    await expect(page.getByText(comment, { exact: true })).toBeVisible();
    await page.getByRole('button', { name: `Удалить расход ${id}`, exact: true }).click();
    await dialog.getByRole('button', { name: 'Удалить', exact: true }).click();
    await expect(page.getByText(comment, { exact: true })).toHaveCount(0);
    expect((await request.get(`/api/expenses/${id}`)).status()).toBe(404);
  } finally {
    if (id) await request.delete(`/api/expenses/${id}`);
  }
});

test('failed loading can be retried', async ({ page }) => {
  await page.route('**/api/expenses?*', route => route.fulfill({ status: 503, body: '{}' }));
  await page.goto('/');
  await expect(page.getByRole('alert')).toContainText('Не удалось загрузить расходы');
  await page.unroute('**/api/expenses?*');
  await page.getByRole('button', { name: 'Повторить', exact: true }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByRole('button', { name: 'Добавить расход' })).toBeEnabled();
});

 test('participant and date filters also restrict totals', async ({ page, request }) => {
  const users = await (await request.get('/api/users')).json();
  expect(users.items.length).toBeGreaterThan(0);
  const userID = users.items[0].id;
  const ids: number[] = [];
  const marker = `Filter test ${Date.now()}`;
  try {
    for (const [date, amount] of [['2099-01-01', '123.45'], ['2099-01-02', '200.00']]) {
      const response = await request.post('/api/expenses', { data: { user_id: userID, amount, category: 'другое', spent_at: date, comment: marker + date } });
      expect(response.status()).toBe(201);
      ids.push((await response.json()).id);
    }
    await page.goto('/');
    await expect(page.getByRole('button', { name: 'Добавить расход' })).toBeEnabled();
    const filters = page.locator('.filters');
    await filters.getByLabel('Участник').selectOption(String(userID));
    await filters.getByLabel('Категория').selectOption('другое');
    await filters.getByLabel('С даты').fill('2099-01-01');
    await filters.getByLabel('По дату').fill('2099-01-01');
    await filters.getByRole('button', { name: 'Показать', exact: true }).click();
    await expect(page.getByText(marker + '2099-01-01', { exact: true })).toBeVisible();
    await expect(page.getByText(marker + '2099-01-02', { exact: true })).toHaveCount(0);
    await expect(page.locator('.primary-stat strong')).toHaveText('123,45 ₽');
  } finally {
    for (const id of ids) await request.delete(`/api/expenses/${id}`);
  }
});
