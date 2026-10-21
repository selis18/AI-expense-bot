import { StrictMode, useEffect, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';

type User = { id: number; display_name: string };
type Expense = { id: number; user_id: number; amount: string; category: string; spent_at: string; comment: string };
type Summary = { count: number; total_kopecks: string; by_category_kopecks: Record<string, string> };
type Filters = { user_id: string; from: string; through: string; category: string };
const categories = ['еда', 'транспорт', 'жильё', 'развлечения', 'здоровье', 'покупки', 'другое'];
const emptyFilters: Filters = { user_id: '', from: '', through: '', category: '' };
const pageSize = 25;

function money(kopecks: string) {
  const value = BigInt(kopecks || '0');
  const rubles = (value / 100n).toLocaleString('ru-RU');
  const fraction = (value % 100n).toString().padStart(2, '0');
  return `${rubles}${fraction === '00' ? '' : `,${fraction}`} ₽`;
}

function amountMoney(value: string) {
  const [rubles, cents = ''] = value.split('.');
  return money(`${rubles}${cents.padEnd(2, '0')}`);
}

async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(`/api${path}`, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  });
  if (!response.ok) {
    const body = await response.json().catch(() => null);
    const messages: Record<string, string> = {
      expense_not_found: 'Расход уже удалён. Обновите список.',
      unknown_user: 'Участник не найден. Обновите страницу.',
      invalid_expense: 'Проверьте сумму, категорию и дату.',
      invalid_query: 'Проверьте период и выбранные фильтры.',
    };
    throw new Error(messages[body?.error?.code] || 'Не удалось выполнить запрос. Попробуйте ещё раз.');
  }
  return response.status === 204 ? undefined as T : response.json();
}

function Modal({ title, children, onClose, busy }: { title: string; children: ReactNode; onClose: () => void; busy: boolean }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => { ref.current?.showModal(); }, []);
  return <dialog ref={ref} aria-labelledby="modal-title" onCancel={event => { event.preventDefault(); if (!busy) onClose(); }}>
    <div className="modal-heading"><h2 id="modal-title">{title}</h2><button className="icon-button" aria-label="Закрыть" disabled={busy} onClick={onClose}>×</button></div>
    {children}
  </dialog>;
}

function ExpenseForm({ expense, users, onClose, onSaved }: { expense: Expense | null; users: User[]; onClose: () => void; onSaved: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const date = new Date();
  const today = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(''); setBusy(true);
    const fields = new FormData(event.currentTarget);
    const payload = {
      amount: String(fields.get('amount')).replace(',', '.'), category: fields.get('category'),
      spent_at: fields.get('spent_at'), comment: fields.get('comment'),
      ...(!expense ? { user_id: Number(fields.get('user_id')) } : {}),
    };
    try {
      await api(expense ? `/expenses/${expense.id}` : '/expenses', { method: expense ? 'PATCH' : 'POST', body: JSON.stringify(payload) });
      onSaved();
    } catch (error) { setError(error instanceof Error ? error.message : 'Не удалось сохранить расход.'); }
    finally { setBusy(false); }
  }
  return <Modal title={expense ? 'Редактировать расход' : 'Новый расход'} onClose={onClose} busy={busy}>
    <form onSubmit={submit}>
      <fieldset disabled={busy}>
        <label>Участник<select name="user_id" defaultValue={expense?.user_id ?? users[0]?.id} disabled={Boolean(expense)} required>{users.map(user => <option key={user.id} value={user.id}>{user.display_name || user.id}</option>)}</select></label>
        <div className="form-grid"><label>Сумма, ₽<input name="amount" inputMode="decimal" pattern="[0-9]+([.,][0-9]{1,2})?" defaultValue={expense?.amount ?? ''} placeholder="0,00" required autoFocus /></label>
          <label>Дата<input name="spent_at" type="date" defaultValue={expense?.spent_at ?? today} required min="0001-01-01" max="9999-12-31" /></label></div>
        <label>Категория<select name="category" defaultValue={expense?.category ?? 'еда'}>{categories.map(category => <option key={category}>{category}</option>)}</select></label>
        <label>Комментарий <span className="optional">необязательно</span><textarea name="comment" maxLength={1000} defaultValue={expense?.comment ?? ''} placeholder="Например, продукты на неделю" rows={3} /></label>
      </fieldset>
      {error && <p role="alert" className="error">{error}</p>}
      <div className="modal-actions"><button type="button" className="secondary" disabled={busy} onClick={onClose}>Отмена</button><button disabled={busy} type="submit">{busy ? 'Сохраняем…' : 'Сохранить'}</button></div>
    </form>
  </Modal>;
}

function DeleteForm({ expense, onClose, onSaved }: { expense: Expense; onClose: () => void; onSaved: () => void }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  async function remove() {
    setBusy(true); setError('');
    try { await api(`/expenses/${expense.id}`, { method: 'DELETE' }); onSaved(); }
    catch (error) { setError(error instanceof Error ? error.message : 'Не удалось удалить расход.'); }
    finally { setBusy(false); }
  }
  return <Modal title="Удалить расход?" busy={busy} onClose={onClose}>
    <p className="delete-description">{expense.comment || expense.category}</p><p className="delete-amount">{amountMoney(expense.amount)}</p>
    <p className="muted">{expense.spent_at.split('-').reverse().join('.')} · {expense.category}</p>
    {error && <p className="error" role="alert">{error}</p>}
    <div className="modal-actions"><button className="secondary" disabled={busy} onClick={onClose}>Отмена</button><button className="danger" disabled={busy} onClick={remove}>{busy ? 'Удаляем…' : 'Удалить'}</button></div>
  </Modal>;
}

function App() {
  const [users, setUsers] = useState<User[]>([]);
  const [expenses, setExpenses] = useState<Expense[]>([]);
  const [summary, setSummary] = useState<Summary>({ count: 0, total_kopecks: '0', by_category_kopecks: {} });
  const [filters, setFilters] = useState<Filters>(emptyFilters);
  const [page, setPage] = useState(0);
  const [revision, setRevision] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [editing, setEditing] = useState<Expense | null | undefined>(undefined);
  const [deleting, setDeleting] = useState<Expense | null>(null);
  const form = useRef<HTMLFormElement>(null);

  useEffect(() => {
    const controller = new AbortController(); setLoading(true); setError('');
    const query = new URLSearchParams(Object.entries(filters).filter(([, value]) => value));
    Promise.all([
      api<{ items: User[] }>('/users', { signal: controller.signal }),
      api<{ items: Expense[] }>(`/expenses?${query}&limit=${pageSize + 1}&offset=${page * pageSize}`, { signal: controller.signal }),
      api<Summary>(`/expenses/summary?${query}`, { signal: controller.signal }),
    ]).then(([members, records, total]) => {
      if (controller.signal.aborted) return;
      setUsers(members.items); setExpenses(records.items); setSummary(total);
    }).catch(error => { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : 'Не удалось загрузить расходы.'); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [filters, page, revision]);

  useEffect(() => { if (!notice) return; const timer = setTimeout(() => setNotice(''), 4000); return () => clearTimeout(timer); }, [notice]);

  function applyFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const data = new FormData(event.currentTarget);
    setFilters({ user_id: String(data.get('user_id')), from: String(data.get('from')), through: String(data.get('through')), category: String(data.get('category')) });
    setPage(0);
  }
  function saved(message: string) {
    setEditing(undefined); setDeleting(null); setNotice(message); setRevision(value => value + 1);
    if (message !== 'Расход сохранён') setPage(0);
  }
  const active = Object.values(filters).some(Boolean);
  const topCategory = Object.entries(summary.by_category_kopecks).sort((a, b) => BigInt(a[1]) > BigInt(b[1]) ? -1 : BigInt(a[1]) < BigInt(b[1]) ? 1 : 0)[0];
  return <div className="app">
    <aside className="sidebar"><a className="brand" href="/"><span className="brand-mark">₽</span><span>Общий счёт<small>Семейные расходы</small></span></a><div className="nav-item"><span>▤</span> Расходы</div><div className="sidebar-note"><span className="status-dot" />Telegram и веб-панель<p>Добавляйте траты в Telegram и управляйте ими здесь.</p></div></aside>
    <main>
      <header><div><p className="eyebrow">СЕМЕЙНЫЙ БЮДЖЕТ</p><h1>Расходы семьи</h1><p className="muted">Добавляйте и редактируйте расходы участников.</p></div><button className="add-button" disabled={loading || users.length === 0 || Boolean(error)} onClick={() => setEditing(null)}><span aria-hidden="true">＋</span> Добавить расход</button></header>
      <section className="stats" aria-label="Итоги по фильтрам">
        <article className="stat primary-stat"><p>{active ? 'По выбранным фильтрам' : 'Всего расходов'}</p><strong>{loading ? '…' : error ? '—' : money(summary.total_kopecks)}</strong><span>Сумма по выбранным фильтрам</span></article>
        <article className="stat"><p>Записей</p><strong>{loading ? '…' : error ? '—' : summary.count}</strong><span>Количество по выбранным фильтрам</span></article>
        <article className="stat"><p>Основная категория</p><strong className="category-total">{loading ? '…' : error ? '—' : topCategory?.[0] || '—'}</strong><span>{!loading && !error && topCategory ? money(topCategory[1]) : 'Появится после добавления трат'}</span></article>
      </section>
      <section className="ledger">
        <div className="section-heading"><h2>Все траты</h2><button className="text-button" onClick={() => setRevision(value => value + 1)} disabled={loading}>↻ Обновить</button></div>
        <form className="filters" ref={form} onSubmit={applyFilters}>
          <label>Участник<select name="user_id"><option value="">Все участники</option>{users.map(user => <option key={user.id} value={user.id}>{user.display_name || user.id}</option>)}</select></label>
          <label>Категория<select name="category"><option value="">Все категории</option>{categories.map(category => <option key={category}>{category}</option>)}</select></label>
          <label>С даты<input name="from" type="date" /></label><label>По дату<input name="through" type="date" /></label>
          <button type="submit" className="secondary" disabled={loading}>Показать</button>
          {active && <button type="button" className="text-button" onClick={() => { form.current?.reset(); setFilters(emptyFilters); setPage(0); }}>Сбросить</button>}
        </form>
        {error ? <div className="empty-state" role="alert"><span className="empty-icon">!</span><h3>Не удалось загрузить расходы</h3><p>{error}</p><button className="secondary" onClick={() => setRevision(value => value + 1)}>Повторить</button></div>
          : loading ? <div className="empty-state" role="status"><span className="loader" /><p>Загружаем расходы…</p></div>
            : expenses.length === 0 ? <div className="empty-state"><span className="empty-icon">₽</span><h3>{active ? 'Нет расходов по этим фильтрам' : 'Здесь появятся ваши расходы'}</h3><p>{active ? 'Выберите другой период, участника или категорию.' : users.length ? 'Добавьте первую трату здесь или напишите боту «кофе 350».' : 'Попросите участника семьи написать боту, чтобы добавить его в список.'}</p></div>
              : <div className="table-scroll"><table><thead><tr><th>Расход</th><th>Участник</th><th>Дата</th><th className="amount-cell">Сумма</th><th><span className="sr-only">Действия</span></th></tr></thead><tbody>{expenses.slice(0, pageSize).map(expense => <tr key={expense.id}>
                <td><div className="expense-title">{expense.comment || expense.category}</div><span className={`category category-${categories.indexOf(expense.category)}`}>{expense.category}</span></td>
                <td><span className="member"><span className="avatar">{(users.find(user => user.id === expense.user_id)?.display_name || '?').slice(0, 1).toUpperCase()}</span>{users.find(user => user.id === expense.user_id)?.display_name || `#${expense.user_id}`}</span></td>
                <td className="date-cell">{expense.spent_at.split('-').reverse().join('.')}</td><td className="amount-cell">{amountMoney(expense.amount)}</td>
                <td className="row-actions"><button className="text-button" aria-label={`Изменить расход ${expense.id}`} onClick={() => setEditing(expense)}>Изменить</button><button className="text-button remove" aria-label={`Удалить расход ${expense.id}`} onClick={() => setDeleting(expense)}>Удалить</button></td>
              </tr>)}</tbody></table></div>}
        <footer className="pagination"><span>{!error && !loading ? `${summary.count} записей · страница ${page + 1}` : 'Расходы семьи'}</span><div><button className="secondary" disabled={loading || page === 0} onClick={() => setPage(value => value - 1)}>Назад</button><button className="secondary" disabled={loading || Boolean(error) || expenses.length <= pageSize} onClick={() => setPage(value => value + 1)}>Далее</button></div></footer>
      </section>
      <p className="page-note">Расходы в рублях · Данные всех участников семьи</p>
      {notice && <div className="toast" role="status">✓ {notice}</div>}
      {editing !== undefined && <ExpenseForm expense={editing} users={users} onClose={() => setEditing(undefined)} onSaved={() => saved(editing ? 'Расход сохранён' : 'Расход добавлен')} />}
      {deleting && <DeleteForm expense={deleting} onClose={() => setDeleting(null)} onSaved={() => saved('Расход удалён')} />}
    </main>
  </div>;
}

createRoot(document.getElementById('root')!).render(<StrictMode><App /></StrictMode>);
