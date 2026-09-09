const tg = window.Telegram?.WebApp;
tg?.ready();
tg?.expand();
tg?.enableClosingConfirmation?.();

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const esc = (value = '') => String(value).replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
const isPreview = new URLSearchParams(location.search).get('preview') === '1';

const i18n = {
  ru: {home:'Главная', tariffs:'Тарифы', support:'Поддержка', more:'Разное', admin:'Админ', expires:'Подписка до', noSubscription:'Нет подписки', devices:'Устройства', traffic:'Трафик', renew:'Продлить', connect:'Подключиться', buy:'Оплатить', days:'дней', unlimited:'Безлимит', createTicket:'Создать тикет', tickets:'Ваши тикеты', serverStatus:'Статус серверов', payments:'Платежи', referral:'Реферальная система'},
  en: {home:'Home', tariffs:'Plans', support:'Support', more:'More', admin:'Admin', expires:'Subscription until', noSubscription:'No subscription', devices:'Devices', traffic:'Traffic', renew:'Renew', connect:'Connect', buy:'Pay', days:'days', unlimited:'Unlimited', createTicket:'Create ticket', tickets:'Your tickets', serverStatus:'Server status', payments:'Payments', referral:'Referral program'}
};

const state = { token: sessionStorage.getItem('tgs_session') || '', data: null, page: new URLSearchParams(location.search).get('page') || 'home', preview: isPreview, cache: {} };
const tr = key => i18n[state.data?.language?.default || 'ru']?.[key] || i18n.ru[key] || key;

const icons = {
  home:'<path d="M3.5 10.5 12 3l8.5 7.5"/><path d="M5.5 9v11h13V9M9 20v-6h6v6"/>',
  plans:'<path d="M4 6.5h16M6 3.5h12a2 2 0 0 1 2 2v13a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2v-13a2 2 0 0 1 2-2Z"/><path d="M8 11h8M8 15h5"/>',
  support:'<path d="M20 15a4 4 0 0 1-4 4H9l-5 3v-7a8 8 0 1 1 16 0Z"/><path d="M9 12a3 3 0 1 1 5.2 2"/><path d="M12 17h.01"/>',
  more:'<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  admin:'<path d="M12 3 4.5 6v5.5c0 4.6 3 7.8 7.5 9.5 4.5-1.7 7.5-4.9 7.5-9.5V6L12 3Z"/><path d="M9.5 12.2 11.2 14l3.7-4"/>',
  gift:'<path d="M4 10h16v10H4zM3 7h18v3H3zM12 7v13M12 7H8.5a2 2 0 1 1 2-2.8L12 7Zm0 0h3.5a2 2 0 1 0-2-2.8L12 7Z"/>',
  devices:'<rect x="5" y="3" width="14" height="18" rx="2"/><path d="M10 18h4"/>',
  traffic:'<path d="M4 18h16M6 15l4-4 3 2 5-7"/><path d="m14 6 4-.5.5 4"/>',
  arrow:'<path d="m9 18 6-6-6-6"/>',
  close:'<path d="m6 6 12 12M18 6 6 18"/>',
  send:'<path d="m21 3-7 18-4-7-7-4 18-7Z"/><path d="m10 14 4-4"/>',
  server:'<rect x="3" y="4" width="18" height="6" rx="2"/><rect x="3" y="14" width="18" height="6" rx="2"/><path d="M7 7h.01M7 17h.01M11 7h6M11 17h6"/>',
  card:'<rect x="3" y="5" width="18" height="14" rx="2"/><path d="M3 10h18M7 15h3"/>',
  referral:'<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M19 8v6M22 11h-6"/>',
  globe:'<circle cx="12" cy="12" r="9"/><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18"/>',
  alert:'<path d="M10.3 4.2 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0Z"/><path d="M12 9v4M12 17h.01"/>',
  pulse:'<path d="M3 12h4l2.5-6 4 12 2.5-6h5"/>',
  toggles:'<path d="M4 7h10M18 7h2M4 17h2M10 17h10"/><circle cx="16" cy="7" r="2"/><circle cx="8" cy="17" r="2"/>',
  clock:'<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
  link:'<path d="M10 13a5 5 0 0 0 7.5.5l2-2a5 5 0 0 0-7-7l-1.2 1.2"/><path d="M14 11a5 5 0 0 0-7.5-.5l-2 2a5 5 0 0 0 7 7l1.2-1.2"/>',
  edit:'<path d="M12 20h9M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4Z"/>',
  palette:'<path d="M12 3a9 9 0 0 0 0 18h1.5a1.5 1.5 0 0 0 0-3H12a2 2 0 0 1 0-4h4a5 5 0 0 0 0-10Z"/><circle cx="7.5" cy="10" r=".7"/><circle cx="10" cy="6.5" r=".7"/>',
  sort:'<path d="M8 6h13M8 12h9M8 18h5M3 5v14M1 17l2 2 2-2"/>',
  user:'<circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/>',
  gear:'<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.6v.2h-4V21a1.7 1.7 0 0 0-1-1.6 1.7 1.7 0 0 0-1.9.3l-.1.1L4.2 17l.1-.1a1.7 1.7 0 0 0 .3-1.9A1.7 1.7 0 0 0 3 14H2.8v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9L4.2 7 7 4.2l.1.1a1.7 1.7 0 0 0 1.9.3A1.7 1.7 0 0 0 10 3V2.8h4V3a1.7 1.7 0 0 0 1 1.6 1.7 1.7 0 0 0 1.9-.3l.1-.1L19.8 7l-.1.1a1.7 1.7 0 0 0-.3 1.9 1.7 1.7 0 0 0 1.6 1h.2v4H21a1.7 1.7 0 0 0-1.6 1Z"/>',
  megaphone:'<path d="m3 11 17-7v16L3 13v-2Z"/><path d="M7 15v4a2 2 0 0 0 2 2h2v-4"/>',
  percent:'<circle cx="12" cy="12" r="9"/><path d="m9 15 6-6M9 9h.01M15 15h.01"/>',
  copy:'<rect x="8" y="8" width="11" height="11" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/>',
  trash:'<path d="M4 7h16M9 7V4h6v3M7 7l1 14h8l1-14M10 11v6M14 11v6"/>',
  plus:'<path d="M12 5v14M5 12h14"/>',
  check:'<path d="m5 12 4 4L19 6"/>',
  back:'<path d="m15 18-6-6 6-6"/>',
  info:'<circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/>'
};
const icon = name => `<svg class="icon" viewBox="0 0 24 24" aria-hidden="true">${icons[name] || icons.info}</svg>`;

function toast(message, error = false) {
  const node = document.createElement('div');
  node.className = `toast${error ? ' error' : ''}`;
  node.textContent = message;
  $('#toast-region').append(node);
  setTimeout(() => node.remove(), 3300);
  tg?.HapticFeedback?.notificationOccurred?.(error ? 'error' : 'success');
}

async function api(path, options = {}) {
  if (state.preview) return previewApi(path, options);
  const headers = {'Content-Type':'application/json', ...(options.headers || {})};
  if (state.token) headers.Authorization = `Bearer ${state.token}`;
  const response = await fetch(path, {...options, headers});
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.detail || 'Не удалось выполнить запрос');
  return data;
}

function applyTheme(theme = {}) {
  const root = document.documentElement;
  const mapping = {accent:'--accent', background:'--bg', surface:'--surface', surface_alt:'--surface-alt', text:'--text', muted:'--muted'};
  Object.entries(mapping).forEach(([key, css]) => { if (/^#[0-9a-f]{6}$/i.test(theme[key] || '')) root.style.setProperty(css, theme[key]); });
  tg?.setHeaderColor?.(theme.background || '#111315');
  tg?.setBackgroundColor?.(theme.background || '#111315');
  tg?.setBottomBarColor?.(theme.background || '#111315');
}

function avatar(user, cls = 'avatar') {
  if (/^https:\/\//.test(user.photo_url || '')) return `<img class="${cls}" src="${esc(user.photo_url)}" alt="">`;
  return `<span class="${cls} avatar-fallback" aria-hidden="true">${esc((user.first_name || 'T')[0].toUpperCase())}</span>`;
}

function formatDate(value) { if (!value) return tr('noSubscription'); return new Intl.DateTimeFormat(state.data?.language?.default === 'en' ? 'en-GB' : 'ru-RU', {day:'numeric', month:'long', year:'numeric'}).format(new Date(value)); }
function bytes(value) { if (!value) return '0 ГБ'; return `${(value / 1073741824).toLocaleString('ru-RU', {maximumFractionDigits:1})} ГБ`; }
function money(value) { return `${Number(value || 0).toLocaleString('ru-RU')} ₽`; }
function statusLabel(status) { return ({ACTIVE:'Активна',INACTIVE:'Не активна',EXPIRED:'Истекла',DISABLED:'Отключена',LIMITED:'Лимит',open:'Открыт',answered:'Ответ получен',closed:'Закрыт',pending:'Ожидает',processing:'Обработка',succeeded:'Оплачен',failed:'Ошибка'}[status] || status); }

function nav() {
  const items = [
    ['home','home',tr('home')], ['tariffs','plans',tr('tariffs')], ['support','support',tr('support')], ['more','more',tr('more')]
  ];
  if (state.data.user.is_admin) items.push(['admin','admin',tr('admin')]);
  const rootPage = state.page.split(':')[0];
  return `<nav class="bottom-nav" style="--nav-count:${items.length}" aria-label="Основная навигация">${items.map(([page,ico,label]) => `<button class="nav-button ${rootPage === page ? 'active' : ''}" data-nav="${page}" aria-current="${rootPage === page ? 'page' : 'false'}">${icon(ico)}<span>${esc(label)}</span></button>`).join('')}</nav>`;
}

function topbar() {
  const content = state.data.content;
  return `<header class="topbar"><div class="topbar-title">${content.logo_url && /^https:\/\//.test(content.logo_url) ? `<img class="topbar-logo" src="${esc(content.logo_url)}" alt="">` : '<span class="topbar-logo" aria-hidden="true">T</span>'}<h1>${esc(content.brand || 'TGS VPN')}</h1></div>${avatar(state.data.user)}</header>`;
}

function render() {
  if (!state.data) return;
  applyTheme(state.data.theme);
  if (state.data.emergency?.enabled && !state.data.user.is_admin) {
    $('#app').innerHTML = `<main class="locked-screen">${icon('alert')}<h1>Технические работы</h1><p>${esc(state.data.content.emergency_message)}</p></main>`;
    return;
  }
  let body;
  if (state.page.startsWith('ticket:')) body = ticketPage(state.page.split(':')[1]);
  else if (state.page.startsWith('more:')) body = moreDetail(state.page.split(':')[1]);
  else if (state.page.startsWith('admin:')) body = adminDetail(state.page.split(':')[1]);
  else body = ({home:homePage, tariffs:tariffsPage, support:supportPage, more:morePage, admin:adminPage}[state.page] || homePage)();
  $('#app').innerHTML = `<div class="app-shell">${topbar()}<main class="page">${body}</main>${nav()}</div>`;
}

function homePage() {
  const {user, trial, features, content} = state.data;
  const sub = user.subscription;
  const active = sub.status === 'ACTIVE' && (!sub.expires_at || new Date(sub.expires_at) > new Date());
  const limit = Number(sub.traffic_limit_bytes || 0), used = Number(sub.traffic_used_bytes || 0);
  const progress = limit ? Math.min(100, used / limit * 100) : 0;
  return `${features.trial && trial.enabled && !user.trial_used ? `<button class="trial-banner" data-action="trial">${icon('gift')}<span><strong>${esc(content.trial_button || 'Бесплатный период')}</strong><span>${trial.days} дней · ${trial.traffic_gb ? `${trial.traffic_gb} ГБ` : tr('unlimited')}</span></span>${icon('arrow')}</button>` : ''}
  <section class="hero">
    <div class="hero-head"><div><p class="eyebrow">${tr('expires')}</p><h2 class="expiry">${formatDate(sub.expires_at)}</h2></div><span class="status ${active ? 'active' : ''}">${active ? statusLabel('ACTIVE') : statusLabel(sub.status)}</span></div>
    <div class="hero-stats"><div class="stat-block"><small>${tr('devices')}</small><strong>${Number(sub.connected_devices || 0)} / ${sub.device_limit || 0}</strong></div><div class="stat-block"><small>${tr('traffic')}</small><strong>${limit ? bytes(limit) : tr('unlimited')}</strong></div></div>
    <div class="traffic-row"><span>Использовано</span><strong>${bytes(used)}${limit ? ` из ${bytes(limit)}` : ''}</strong></div><div class="progress" role="progressbar" aria-valuenow="${Math.round(progress)}" aria-valuemin="0" aria-valuemax="100"><i style="width:${progress}%"></i></div>
    <div class="action-row"><button class="secondary" data-nav="tariffs">${icon('plans')}${esc(content.renew_button || tr('renew'))}</button><button class="primary" data-action="connect" ${sub.subscription_url ? '' : 'disabled'}>${icon('link')}${esc(content.connect_button || tr('connect'))}</button></div>
  </section>
  <div class="section-title"><h3>Быстрый доступ</h3></div><div class="list">
    ${row('plans','Тарифы','Выбрать или продлить подписку','tariffs')}${features.support ? row('support','Поддержка','Тикеты и чат','support') : ''}${features.devices ? row('devices','Устройства','Просмотр и управление','more:devices') : ''}
  </div>`;
}

function row(ico,title,subtitle,target,action='nav') { return `<button class="list-row" data-${action}="${esc(target)}"><span class="icon-box">${icon(ico)}</span><span><strong>${esc(title)}</strong><small>${esc(subtitle)}</small></span><span class="chevron">${icon('arrow')}</span></button>`; }

function tariffsPage() {
  const tariffs = state.data.tariffs || [];
  return `<div class="page-heading"><h2>${tr('tariffs')}</h2><p>Выберите срок и нужные лимиты</p></div><div class="tariff-list">${tariffs.length ? tariffs.map(t => `<article class="tariff ${t.pinned ? 'pinned' : ''}">${t.pinned ? '<span class="tariff-badge">Выгодно</span>' : ''}<h3>${esc(t.name)}</h3><p>${esc(t.description)}</p><div class="tariff-meta"><span>${icon('clock')}${t.days} ${tr('days')}</span><span>${icon('devices')}${t.device_limit} устр.</span><span>${icon('traffic')}${t.traffic_gb ? `${t.traffic_gb} ГБ` : tr('unlimited')}</span></div><div class="tariff-footer"><span class="price">${money(t.price_rub)}</span><button class="primary" data-buy="${t.id}">${tr('buy')}</button></div></article>`).join('') : empty('plans','Тарифы пока не добавлены')}</div>`;
}

function supportPage() {
  const tickets = state.data.tickets || [];
  return `<div class="page-heading"><h2>${tr('support')}</h2><p>${esc(state.data.content.support_welcome)}</p></div><button class="primary" style="width:100%;margin-bottom:14px" data-action="new-ticket">${icon('plus')}${tr('createTicket')}</button><div class="section-title"><h3>${tr('tickets')}</h3><span>${tickets.length}</span></div><div class="list">${tickets.length ? tickets.map(t => `<button class="list-row ticket-row" data-nav="ticket:${t.id}"><span class="ticket-copy"><strong>${esc(t.subject)}</strong><small>${formatDate(t.updated_at)}</small></span><span class="status ${esc(t.status)}">${statusLabel(t.status)}</span></button>`).join('') : empty('support','Открытых тикетов нет')}</div>`;
}

function ticketPage(id) {
  const ticket = (state.page.startsWith('admin:') ? state.cache.adminTickets : state.data.tickets)?.find(t => t.id === id);
  if (!ticket) return `<div class="page-heading"><h2>Тикет</h2></div>${empty('support','Тикет не найден')}`;
  return `<div class="page-heading"><button class="text-button" data-nav="support">${icon('back')} Назад</button><h2>${esc(ticket.subject)}</h2><p>${statusLabel(ticket.status)}</p></div><div class="chat">${ticket.messages.map(m => `<div class="bubble ${m.is_admin ? '' : 'mine'}">${esc(m.text)}<small>${formatDate(m.created_at)}</small></div>`).join('')}</div>${ticket.status !== 'closed' ? `<form class="chat-compose" id="chat-form"><input name="text" maxlength="5000" aria-label="Сообщение" placeholder="Сообщение…" autocomplete="off"><button class="icon-button" aria-label="Отправить">${icon('send')}</button></form>` : ''}`;
}

const moreItems = {
  servers:['server','Статус серверов','Доступность нод Remnawave'], devices:['devices','Устройства','Управление подключениями'], payments:['card','Платежи','История покупок'], referral:['referral','Реферальная система','Ссылка и вознаграждения']
};
function morePage() {
  const order = state.data.more_order || Object.keys(moreItems), f = state.data.features;
  const enabled = {servers:f.server_status, devices:f.devices, payments:f.payments, referral:f.referrals};
  return `<div class="page-heading"><h2>${tr('more')}</h2><p>Профиль и сервисы</p></div><section class="hero" style="margin-bottom:14px"><div style="display:flex;align-items:center;gap:13px">${avatar(state.data.user,'avatar')}<div><strong style="font-size:18px">${esc(state.data.user.first_name)}</strong><div style="color:var(--muted);font-size:13px;margin-top:3px">${state.data.user.username ? '@'+esc(state.data.user.username) : 'Telegram ID '+state.data.user.telegram_id}</div></div></div></section><div class="list">${order.filter(key => enabled[key] && moreItems[key]).map(key => row(moreItems[key][0],moreItems[key][1],moreItems[key][2],`more:${key}`)).join('')}</div>`;
}

function moreDetail(key) {
  const labels = moreItems[key] || ['info','Раздел',''];
  const back = `<button class="text-button" data-nav="more">${icon('back')} Назад</button>`;
  if (key === 'payments') return `<div class="page-heading">${back}<h2>${labels[1]}</h2></div><div class="list">${state.data.payments?.length ? state.data.payments.map(p => `<div class="list-row" style="cursor:default"><span class="icon-box">${icon('card')}</span><span><strong>${money(p.amount_rub)}</strong><small>${esc(p.snapshot?.name || p.provider)} · ${formatDate(p.created_at)}</small></span><span class="status ${p.status}">${statusLabel(p.status)}</span></div>`).join('') : empty('card','Платежей пока нет')}</div>`;
  if (key === 'referral') { const ref = state.data.referral; return `<div class="page-heading">${back}<h2>${labels[1]}</h2><p>Приглашайте друзей и получайте бонус после их первой покупки</p></div><section class="hero"><p class="eyebrow">Приглашено</p><h2 class="expiry">${ref.count}</h2><div class="hero-stats"><div class="stat-block"><small>Бонус</small><strong>${ref.referral_days} дней</strong></div><div class="stat-block"><small>Трафик</small><strong>${ref.referral_traffic_gb} ГБ</strong></div></div><div class="code">${esc(ref.link)}</div><button class="primary" style="width:100%;margin-top:12px" data-copy="${esc(ref.link)}">${icon('copy')}Скопировать ссылку</button></section>`; }
  const cached = state.cache[key];
  if (!cached) { loadMore(key); return `<div class="page-heading">${back}<h2>${labels[1]}</h2></div><div class="skeleton"></div>`; }
  if (key === 'servers') return `<div class="page-heading">${back}<h2>${labels[1]}</h2><p>Данные напрямую из панели</p></div><div class="list">${cached.length ? cached.map(n => `<div class="list-row" style="cursor:default"><span class="icon-box">${icon('server')}</span><span><strong>${esc(n.name || 'Node')}</strong><small>${esc(n.country_code || 'Локация не указана')}</small></span><span class="status ${n.status}">${n.status === 'online' ? 'Работает' : 'Недоступен'}</span></div>`).join('') : empty('server','Ноды не найдены')}</div>`;
  if (key === 'devices') return `<div class="page-heading">${back}<h2>${labels[1]}</h2><p>Удаляйте только незнакомые устройства</p></div><div class="list">${cached.length ? cached.map((d,idx) => `<div class="list-row"><span class="icon-box">${icon('devices')}</span><span><strong>${esc(d.deviceModel || d.platform || `Устройство ${idx+1}`)}</strong><small>${esc(d.userAgent || d.hwid || 'HWID')}</small></span><button class="compact-button danger" data-delete-device="${esc(d.hwid || '')}" aria-label="Удалить устройство">${icon('trash')}</button></div>`).join('') : empty('devices','Подключённых устройств нет')}</div>`;
  return '';
}

async function loadMore(key) { try { state.cache[key] = await api(key === 'servers' ? '/api/nodes' : '/api/devices'); render(); } catch (e) { state.cache[key] = []; toast(e.message, true); render(); } }

function adminPage() {
  const items = [
    ['globe','Язык','Язык интерфейса','language'], ['alert','Режим аварии','Отключить доступ пользователям','emergency'], ['pulse','Диагностика','Ошибки панели и платежей','diagnostics'], ['toggles','Управление функциями','Включение разделов','features'], ['gift','Триал','Срок, трафик и сквады','trial'], ['link','Интеграции','Remnawave и платежи','integrations'], ['edit','Редактор контента','Тексты и брендинг','content'], ['palette','Оформление','Цветовые шаблоны','theme'], ['sort','Разное','Порядок пунктов','more_order'], ['plans','Тарифы','Каталог и сквады','tariffs'], ['user','Пользователи','Поиск и управление','users'], ['support','Тикеты поддержки','Ответы пользователям','tickets'], ['gear','Система','Реферальные бонусы','system'], ['megaphone','Рассылка','Создание через Telegram','broadcast'], ['percent','Промокоды','Скидки и лимиты','promos']
  ];
  if (!state.cache.overview) loadAdmin('overview');
  const o = state.cache.overview;
  return `<div class="page-heading"><h2>Админ</h2><p>Управление TGS VPN</p></div>${o ? `<div class="admin-metrics"><div class="metric"><span>Пользователи</span><strong>${o.users}</strong></div><div class="metric"><span>Активные</span><strong>${o.active_subscriptions}</strong></div><div class="metric"><span>Выручка</span><strong>${money((o.revenue_kopecks || 0)/100)}</strong></div><div class="metric"><span>Ошибки</span><strong>${o.diagnostics}</strong></div></div>` : '<div class="skeleton"></div>'}<div class="list">${items.map(([ico,title,sub,key]) => row(ico,title,sub,`admin:${key}`)).join('')}</div>`;
}

async function loadAdmin(key, path = `/api/admin/${key}`) { try { state.cache[key] = await api(path); render(); } catch (e) { state.cache[key] = {error:e.message}; toast(e.message,true); render(); } }
function adminBack(title, subtitle='') { return `<div class="page-heading"><button class="text-button" data-nav="admin">${icon('back')} Назад</button><h2>${esc(title)}</h2>${subtitle ? `<p>${esc(subtitle)}</p>` : ''}</div>`; }

function adminDetail(key) {
  const settingKeys = ['language','emergency','features','trial','integrations','content','theme','more_order','system'];
  if (settingKeys.includes(key)) {
    if (!state.cache[`setting:${key}`]) { loadAdminSetting(key); return `${adminBack('Настройки')}<div class="skeleton"></div>`; }
    return settingPage(key, state.cache[`setting:${key}`]);
  }
  if (key === 'diagnostics') return adminCollection(key,'Диагностика','/api/admin/diagnostics',diagnosticsView);
  if (key === 'tariffs') return adminCollection(key,'Тарифы','/api/admin/tariffs',adminTariffsView);
  if (key === 'users') return adminCollection(key,'Пользователи','/api/admin/users',adminUsersView);
  if (key === 'tickets') return adminCollection(key,'Тикеты поддержки','/api/admin/tickets',adminTicketsView);
  if (key === 'promos') return adminCollection(key,'Промокоды','/api/admin/promos',adminPromosView);
  if (key === 'broadcast') return `${adminBack('Рассылка','Сообщение вводится в Telegram')}<div class="admin-form"><h3>Новая рассылка</h3><p class="notice">Отправьте боту команду <strong>/broadcast</strong>, затем текст. Кнопки добавляются отдельными строками: «Название | https://ссылка». Бот покажет предпросмотр и попросит подтверждение.</p><div class="code">/broadcast Новость для пользователей<br><br>Открыть сайт | https://example.com</div></div>`;
  return adminBack('Раздел');
}

async function loadAdminSetting(key) { try { state.cache[`setting:${key}`] = await api(`/api/admin/settings/${key}`); render(); } catch(e) { toast(e.message,true); } }
function adminCollection(key,title,path,view) { if (!state.cache[`admin:${key}`]) { loadAdmin(`admin:${key}`,path); return `${adminBack(title)}<div class="skeleton"></div>`; } const data=state.cache[`admin:${key}`]; return adminBack(title)+view(data); }

function switchRow(name,label,checked,description='') { return `<div class="switch-row"><span><strong>${esc(label)}</strong>${description ? `<small style="display:block;color:var(--muted);margin-top:3px">${esc(description)}</small>` : ''}</span><label class="switch"><input type="checkbox" name="${esc(name)}" ${checked ? 'checked' : ''}><i></i></label></div>`; }
function field(name,label,value='',type='text',hint='',wide=true) { return `<div class="field ${wide ? 'wide' : ''}"><label for="f-${esc(name)}">${esc(label)}</label><input id="f-${esc(name)}" name="${esc(name)}" type="${type}" value="${esc(value)}">${hint ? `<p class="field-hint">${esc(hint)}</p>` : ''}</div>`; }

function settingPage(key, payload) {
  const v = payload.value || {}, titles = {language:'Язык',emergency:'Режим аварии',features:'Управление функциями',trial:'Триал',integrations:'Интеграции',content:'Редактор контента',theme:'Оформление',more_order:'Разное',system:'Система'};
  let inner='';
  if (key === 'language') inner = `<div class="field"><label>Язык всего интерфейса</label><select name="default"><option value="ru" ${v.default==='ru'?'selected':''}>Русский</option><option value="en" ${v.default==='en'?'selected':''}>English</option></select></div>`;
  if (key === 'emergency') inner = switchRow('enabled','Аварийный режим',v.enabled,'Пользователи увидят только сообщение о работах') + '<p class="notice">Текст аварии меняется в «Редакторе контента».</p>';
  if (key === 'features') inner = Object.entries({trial:'Бесплатный период',server_status:'Статус серверов',devices:'Устройства',payments:'Платежи',referrals:'Реферальная система',promo_codes:'Промокоды',support:'Поддержка'}).map(([n,l])=>switchRow(n,l,v[n])).join('');
  if (key === 'trial') inner = `<div class="form-grid">${switchRow('enabled','Триал включён',v.enabled)}${field('days','Количество дней',v.days,'number','',false)}${field('traffic_gb','Трафик, ГБ (0 = безлимит)',v.traffic_gb,'number','',false)}${field('device_limit','Устройств',v.device_limit,'number','',false)}${field('internal_squads','UUID внутренних сквадов',Array.isArray(v.internal_squads)?v.internal_squads.join(', '):'','text','Через запятую')}${field('external_squad_uuid','UUID внешнего сквада',v.external_squad_uuid||'')}</div>`;
  if (key === 'content') inner = `<div class="form-grid">${[['brand','Название'],['logo_url','URL логотипа'],['start_title','Заголовок /start'],['start_text','Текст /start'],['trial_button','Кнопка триала'],['cabinet_button','Кнопка кабинета'],['support_button','Кнопка поддержки'],['connect_button','Кнопка подключения'],['renew_button','Кнопка продления'],['support_welcome','Текст поддержки'],['emergency_message','Сообщение аварии']].map(([n,l])=>field(n,l,v[n]||'')).join('')}</div>`;
  if (key === 'system') inner = `<div class="form-grid">${field('referral_days','Бонус дней',v.referral_days,'number','',false)}${field('referral_traffic_gb','Бонус трафика, ГБ',v.referral_traffic_gb,'number','',false)}${switchRow('reward_after_payment','Только после первой оплаты',v.reward_after_payment)}</div>`;
  if (key === 'more_order') { const items=v.items||[]; inner=`<p class="notice">Изменяйте порядок кнопками. Видимость разделов задаётся в управлении функциями.</p><div class="list" id="order-list">${items.map((name,idx)=>`<div class="list-row" data-order-item="${name}"><span class="icon-box">${icon(moreItems[name]?.[0]||'info')}</span><span><strong>${esc(moreItems[name]?.[1]||name)}</strong></span><span><button type="button" class="compact-button" data-move="${idx}:up" aria-label="Выше">↑</button> <button type="button" class="compact-button" data-move="${idx}:down" aria-label="Ниже">↓</button></span></div>`).join('')}</div><input type="hidden" name="items" value="${esc(items.join(','))}">`; }
  if (key === 'theme') { const templates=payload.theme_templates||{}; inner=`<div class="theme-grid">${Object.entries(templates).map(([name,t])=>`<button type="button" class="theme-option ${v.template===name?'active':''}" data-theme="${name}" data-theme-values="${esc(JSON.stringify(t))}"><span class="swatches"><i style="background:${t.background}"></i><i style="background:${t.surface}"></i><i style="background:${t.accent}"></i></span><strong>${esc(({telegram:'Telegram',graphite:'Графит',emerald:'Изумруд',sand:'Песок'}[name]||name))}</strong></button>`).join('')}</div><input type="hidden" name="template" value="${esc(v.template||'telegram')}">${['accent','background','surface','surface_alt','text','muted'].map(n=>field(n,n,v[n]||'', 'text','',true)).join('')}`; }
  if (key === 'integrations') { inner = integrationForms(v); }
  return `${adminBack(titles[key])}<form class="admin-form" data-setting-form="${key}">${inner}<button class="primary" style="width:100%;margin-top:16px">Сохранить</button></form>`;
}

function integrationForms(v) {
  const groups=[
    ['remnawave','Remnawave',[['url','URL панели','url'],['token','API Token','password'],['webhook_secret','Секрет webhook','password']]],
    ['yookassa','ЮKassa',[['shop_id','Shop ID','text'],['secret_key','Secret Key','password'],['email','Email для чеков','email']]],
    ['cryptobot','CryptoBot',[['token','API Token','password']]],
    ['notifications','Бот уведомлений',[['bot_token','Токен бота','password'],['chat_id','Chat ID','text']]]
  ];
  return groups.map(([key,title,fields])=>{const c=v[key]||{};const webhookHint=key==='remnawave'?'<p class="field-hint">Webhook: /api/webhooks/remnawave · заголовок X-Webhook-Secret</p>':'';return `<section class="admin-form" style="margin-bottom:10px"><h3>${title}</h3>${switchRow(`${key}.enabled`,'Включено',c.enabled)}${fields.map(([n,l,t])=>field(`${key}.${n}`,l,c[n]||'',t)).join('')}${webhookHint}${key==='cryptobot'?switchRow('cryptobot.testnet','Тестовая сеть',c.testnet):''}${key!=='notifications'?`<button type="button" class="compact-button" data-test-integration="${key}">Проверить подключение</button>`:''}</section>`}).join('');
}

function diagnosticsView(rows) { if (!Array.isArray(rows)) return empty('alert',rows.error||'Ошибка'); return `<div class="list">${rows.length ? rows.map(d=>`<div class="list-row"><span class="icon-box">${icon('pulse')}</span><span><strong>${esc(d.source)}</strong><small>${esc(d.message)} · ${formatDate(d.created_at)}</small></span>${d.resolved?'<span class="status active">Решено</span>':`<button class="compact-button" data-resolve="${d.id}">Решено</button>`}</div>`).join('') : empty('pulse','Ошибок нет')}</div>`; }
function adminTariffsView(rows) { if (!Array.isArray(rows)) return empty('plans','Ошибка'); return `<button class="primary" style="width:100%;margin-bottom:12px" data-admin-tariff="new">${icon('plus')}Создать тариф</button><div class="list">${rows.map(t=>`<button class="list-row" data-admin-tariff="${t.id}"><span class="icon-box">${icon('plans')}</span><span><strong>${esc(t.name)}</strong><small>${t.days} дней · ${money(t.price_rub)}${t.active?'':' · выключен'}</small></span>${icon('arrow')}</button>`).join('')}</div>`; }
function adminUsersView(rows) { if (!Array.isArray(rows)) return empty('user','Ошибка'); return `<form id="user-search" class="field" style="margin-top:0"><label>Поиск</label><input name="q" placeholder="Имя, username или Telegram ID"></form><div class="list">${rows.length?rows.map(u=>`<button class="list-row" data-admin-user="${u.id}">${avatar(u,'avatar')}<span><strong>${esc(u.first_name)}</strong><small>${u.username?'@'+esc(u.username):u.telegram_id} · ${statusLabel(u.subscription.status)}</small></span>${icon('arrow')}</button>`).join(''):empty('user','Пользователи не найдены')}</div>`; }
function adminTicketsView(rows) { if (!Array.isArray(rows)) return empty('support','Ошибка'); return `<div class="list">${rows.length?rows.map(t=>`<button class="list-row ticket-row" data-admin-ticket="${t.id}"><span><strong>${esc(t.subject)}</strong><small>${esc(t.user?.first_name||'')} · ${formatDate(t.updated_at)}</small></span><span class="status ${t.status}">${statusLabel(t.status)}</span></button>`).join(''):empty('support','Тикетов нет')}</div>`; }
function adminPromosView(rows) { if (!Array.isArray(rows)) return empty('percent','Ошибка'); return `<button class="primary" style="width:100%;margin-bottom:12px" data-action="new-promo">${icon('plus')}Создать промокод</button><div class="list">${rows.length?rows.map(p=>`<div class="list-row"><span class="icon-box">${icon('percent')}</span><span><strong>${esc(p.code)} · ${p.discount_percent}%</strong><small>${p.uses}${p.max_uses?` / ${p.max_uses}`:''} использований</small></span><button class="compact-button danger" data-delete-promo="${p.id}" aria-label="Удалить">${icon('trash')}</button></div>`).join(''):empty('percent','Промокодов нет')}</div>`; }

function empty(ico,text) { return `<div class="empty">${icon(ico)}${esc(text)}</div>`; }

function openModal(title, content, actions = '') {
  $('#modal-root').innerHTML = `<div class="modal-backdrop" data-modal-backdrop><section class="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title"><header class="modal-head"><h3 id="modal-title">${esc(title)}</h3><button class="modal-close" data-close-modal aria-label="Закрыть">${icon('close')}</button></header>${content}${actions}</section></div>`;
  setTimeout(() => $('.modal input, .modal select, .modal button')?.focus(), 30);
}
function closeModal() { $('#modal-root').innerHTML = ''; }

function buyModal(id) {
  const tariff=state.data.tariffs.find(t=>t.id===id), methods=state.data.payment_methods.filter(m=>m.enabled);
  openModal(`Оплата — ${tariff.name}`, `<form id="checkout-form"><input type="hidden" name="tariff_id" value="${tariff.id}"><div class="choice-list">${methods.length?methods.map((m,i)=>`<label class="choice"><input type="radio" name="provider" value="${m.id}" ${i===0?'checked':''}><span><strong>${esc(m.name)}</strong><small style="display:block;color:var(--muted);margin-top:3px">${m.id==='yookassa'?'Банковская карта или СБП':'Криптовалюта через Telegram'}</small></span></label>`).join(''):empty('card','Способы оплаты ещё не подключены')}</div>${state.data.features.promo_codes?field('promo_code','Промокод','','text','Необязательно'):''}<button class="primary" style="width:100%;margin-top:15px" ${methods.length?'':'disabled'}>Оплатить ${money(tariff.price_rub)}</button></form>`);
}
function ticketModal() { openModal('Новый тикет', `<form id="ticket-form">${field('subject','Тема','','text','Коротко опишите вопрос')}<div class="field"><label for="ticket-description">Описание</label><textarea id="ticket-description" name="description" maxlength="5000" required placeholder="Что случилось?"></textarea></div><button class="primary" style="width:100%">Отправить</button></form>`); }

function tariffAdminModal(id) { const existing=id==='new'?{name:'',description:'',price_rub:199,days:30,traffic_gb:100,device_limit:1,internal_squads:[],external_squad_uuid:'',active:true,pinned:false,position:0}:state.cache['admin:tariffs'].find(t=>t.id===id);openModal(id==='new'?'Новый тариф':'Редактирование тарифа',`<form id="tariff-admin-form" data-id="${id}"><div class="form-grid">${field('name','Название',existing.name)}${field('description','Описание',existing.description)}${field('price_rub','Цена, ₽',existing.price_rub,'number','',false)}${field('days','Дней',existing.days,'number','',false)}${field('traffic_gb','Трафик, ГБ (0 = безлимит)',existing.traffic_gb,'number','',false)}${field('device_limit','Устройств',existing.device_limit,'number','',false)}${field('internal_squads','UUID сквадов',existing.internal_squads.join(', '),'text','Через запятую')}${field('external_squad_uuid','Внешний сквад',existing.external_squad_uuid||'')}</div>${switchRow('active','Тариф включён',existing.active)}${switchRow('pinned','Закрепить первым',existing.pinned)}${field('position','Позиция',existing.position,'number')}<div class="modal-actions">${id!=='new'?`<button type="button" class="danger-button" data-remove-tariff="${id}">Удалить</button>`:'<button type="button" class="secondary" data-close-modal>Отмена</button>'}<button class="primary">Сохранить</button></div></form>`); }
function userAdminModal(id){const u=state.cache['admin:users'].find(x=>String(x.id)===String(id));openModal(u.first_name,`<form id="user-admin-form" data-id="${u.id}"><div class="code">Telegram ID: ${u.telegram_id}<br>${u.username?'@'+esc(u.username):'Без username'}<br>Подписка до: ${formatDate(u.subscription.expires_at)}<br>Рефералов: ${u.referral_count||0} · код ${esc(u.referral_code||'—')}</div><div class="field"><label>Статус подписки</label><select name="subscription_status"><option value="">Не менять</option><option value="ACTIVE">Активна</option><option value="DISABLED">Отключена</option></select></div><div class="form-grid">${field('add_days','Добавить дней',0,'number','',false)}${field('add_traffic_gb','Добавить ГБ',0,'number','',false)}${field('device_limit','Лимит устройств',u.subscription.device_limit,'number','',false)}</div>${switchRow('blocked','Заблокирован',u.is_blocked)}${switchRow('admin','Администратор',u.is_admin)}<button class="primary" style="width:100%;margin-top:14px">Сохранить</button></form>`)}
function promoModal(){openModal('Новый промокод',`<form id="promo-form">${field('code','Код','','text')}${field('discount_percent','Скидка, %',10,'number')}${field('max_uses','Лимит использований (0 = без лимита)',0,'number')}${switchRow('active','Активен',true)}<button class="primary" style="width:100%;margin-top:14px">Создать</button></form>`)}

document.addEventListener('click', async event => {
  const navButton=event.target.closest('[data-nav]'); if(navButton){state.page=navButton.dataset.nav;state.cache={...state.cache,overview:state.cache.overview};history.replaceState(null,'',`/app?page=${encodeURIComponent(state.page)}${state.preview?'&preview=1':''}`);render();return;}
  if(event.target.closest('[data-close-modal]') || (event.target.matches('[data-modal-backdrop]'))){closeModal();return;}
  const buy=event.target.closest('[data-buy]');if(buy){buyModal(buy.dataset.buy);return;}
  const action=event.target.closest('[data-action]')?.dataset.action;
  if(action==='new-ticket'){ticketModal();return;} if(action==='new-promo'){promoModal();return;}
  if(action==='trial'){try{await api('/api/trial',{method:'POST',body:'{}'});await loadBootstrap();toast('Пробный период активирован');}catch(e){toast(e.message,true)}return;}
  if(action==='connect'){const link=state.data.user.subscription.subscription_url;if(link){tg?.openLink?tg.openLink(link):window.open(link,'_blank')}return;}
  const copy=event.target.closest('[data-copy]');if(copy){await navigator.clipboard.writeText(copy.dataset.copy);toast('Ссылка скопирована');return;}
  const device=event.target.closest('[data-delete-device]');if(device&&confirm('Удалить это устройство?')){try{await api(`/api/devices/${encodeURIComponent(device.dataset.deleteDevice)}`,{method:'DELETE'});state.cache.devices=null;loadMore('devices');toast('Устройство удалено');}catch(e){toast(e.message,true)}return;}
  const tariff=event.target.closest('[data-admin-tariff]');if(tariff){tariffAdminModal(tariff.dataset.adminTariff);return;}
  const user=event.target.closest('[data-admin-user]');if(user){userAdminModal(user.dataset.adminUser);return;}
  const ticket=event.target.closest('[data-admin-ticket]');if(ticket){adminTicketModal(ticket.dataset.adminTicket);return;}
  if(event.target.closest('[data-remove-tariff]')){const id=event.target.closest('[data-remove-tariff]').dataset.removeTariff;if(confirm('Удалить тариф?')){await api(`/api/admin/tariffs/${id}`,{method:'DELETE'});closeModal();state.cache['admin:tariffs']=null;loadAdmin('admin:tariffs','/api/admin/tariffs');toast('Тариф удалён')}return;}
  const promo=event.target.closest('[data-delete-promo]');if(promo&&confirm('Удалить промокод?')){await api(`/api/admin/promos/${promo.dataset.deletePromo}`,{method:'DELETE'});state.cache['admin:promos']=null;loadAdmin('admin:promos','/api/admin/promos');toast('Промокод удалён');return;}
  const resolve=event.target.closest('[data-resolve]');if(resolve){await api(`/api/admin/diagnostics/${resolve.dataset.resolve}`,{method:'PATCH',body:'{}'});state.cache['admin:diagnostics']=null;loadAdmin('admin:diagnostics','/api/admin/diagnostics');return;}
  const test=event.target.closest('[data-test-integration]');if(test){try{const value=await api(`/api/admin/integrations/${test.dataset.testIntegration}/test`,{method:'POST',body:'{}'});toast(value.message)}catch(e){toast(e.message,true)}return;}
  const theme=event.target.closest('[data-theme]');if(theme){const form=theme.closest('form');const values=JSON.parse(theme.dataset.themeValues);form.elements.template.value=theme.dataset.theme;Object.entries(values).forEach(([k,v])=>{if(form.elements[k])form.elements[k].value=v});applyTheme({...state.data.theme,...values});$$('.theme-option',form).forEach(x=>x.classList.toggle('active',x===theme));return;}
  const move=event.target.closest('[data-move]');if(move){const [idxRaw,dir]=move.dataset.move.split(':');const input=move.closest('form').elements.items;const items=input.value.split(',');const idx=Number(idxRaw),next=dir==='up'?idx-1:idx+1;if(next>=0&&next<items.length){[items[idx],items[next]]=[items[next],items[idx]];input.value=items.join(',');state.cache['setting:more_order'].value.items=items;render()}return;}
});

document.addEventListener('submit', async event => {
  event.preventDefault(); const form=event.target;
  try {
    if(form.id==='checkout-form'){const body=Object.fromEntries(new FormData(form));const out=await api('/api/payments/checkout',{method:'POST',body:JSON.stringify(body)});closeModal();tg?.openLink?tg.openLink(out.pay_url):window.open(out.pay_url,'_blank');toast(`Счёт на ${money(out.amount_rub)} создан`);return;}
    if(form.id==='ticket-form'){const body=Object.fromEntries(new FormData(form));const ticket=await api('/api/tickets',{method:'POST',body:JSON.stringify(body)});state.data.tickets.unshift(ticket);closeModal();state.page=`ticket:${ticket.id}`;render();toast('Тикет создан');return;}
    if(form.id==='chat-form'){const input=form.elements.text;if(!input.value.trim())return;const id=state.page.split(':')[1];const ticket=await api(`/api/tickets/${id}/messages`,{method:'POST',body:JSON.stringify({text:input.value.trim()})});const idx=state.data.tickets.findIndex(t=>t.id===id);state.data.tickets[idx]=ticket;render();return;}
    if(form.dataset.settingForm){const key=form.dataset.settingForm,body=collectForm(form);const out=await api(`/api/admin/settings/${key}`,{method:'PUT',body:JSON.stringify({value:body})});state.cache[`setting:${key}`]=out;if(key==='theme'){state.data.theme={...state.data.theme,...out.value};applyTheme(state.data.theme)}if(key==='language'){state.data.language=out.value}if(key==='emergency'){state.data.emergency=out.value}if(key==='features'){state.data.features={...state.data.features,...out.value}}if(key==='more_order'){state.data.more_order=out.value.items}toast('Сохранено');render();return;}
    if(form.id==='tariff-admin-form'){const id=form.dataset.id,body=collectForm(form);body.price_rub=Number(body.price_rub);['days','traffic_gb','device_limit','position'].forEach(k=>body[k]=Number(body[k]));body.internal_squads=(body.internal_squads||'').split(',').map(x=>x.trim()).filter(Boolean);const method=id==='new'?'POST':'PUT',path=id==='new'?'/api/admin/tariffs':`/api/admin/tariffs/${id}`;await api(path,{method,body:JSON.stringify(body)});closeModal();state.cache['admin:tariffs']=null;await loadAdmin('admin:tariffs','/api/admin/tariffs');toast('Тариф сохранён');return;}
    if(form.id==='user-admin-form'){const body=collectForm(form);['add_days','add_traffic_gb','device_limit'].forEach(k=>body[k]=Number(body[k]));await api(`/api/admin/users/${form.dataset.id}`,{method:'PATCH',body:JSON.stringify(body)});closeModal();state.cache['admin:users']=null;await loadAdmin('admin:users','/api/admin/users');toast('Пользователь обновлён');return;}
    if(form.id==='promo-form'){const body=collectForm(form);body.discount_percent=Number(body.discount_percent);body.max_uses=Number(body.max_uses);await api('/api/admin/promos',{method:'POST',body:JSON.stringify(body)});closeModal();state.cache['admin:promos']=null;await loadAdmin('admin:promos','/api/admin/promos');toast('Промокод создан');return;}
    if(form.id==='user-search'){state.cache['admin:users']=null;await loadAdmin('admin:users',`/api/admin/users?q=${encodeURIComponent(form.elements.q.value)}`);return;}
    if(form.id==='admin-ticket-chat'){const id=form.dataset.id,textValue=form.elements.text.value.trim();if(!textValue)return;await api(`/api/tickets/${id}/messages`,{method:'POST',body:JSON.stringify({text:textValue})});state.cache['admin:tickets']=await api('/api/admin/tickets');closeModal();adminTicketModal(id);return;}
  } catch(e) { toast(e.message,true); }
});

function collectForm(form) { const out={};$$('[name]',form).forEach(input=>{let value=input.type==='checkbox'?input.checked:input.value;if(input.type==='number')value=Number(value);const path=input.name.split('.');let cursor=out;path.forEach((part,idx)=>{if(idx===path.length-1)cursor[part]=value;else cursor=cursor[part]||(cursor[part]={})})});if(form.dataset.settingForm==='more_order')out.items=(out.items||'').split(',').filter(Boolean);if(form.dataset.settingForm==='trial')out.internal_squads=(out.internal_squads||'').split(',').map(x=>x.trim()).filter(Boolean);return out;}

function adminTicketModal(id){const ticket=state.cache['admin:tickets'].find(t=>t.id===id);openModal(ticket.subject,`<div class="chat" style="padding-bottom:8px;max-height:48vh;overflow:auto">${ticket.messages.map(m=>`<div class="bubble ${m.is_admin?'mine':''}">${esc(m.text)}<small>${formatDate(m.created_at)}</small></div>`).join('')}</div><form id="admin-ticket-chat" data-id="${id}"><div class="field"><label>Ответ</label><textarea name="text" maxlength="5000"></textarea></div><button class="primary" style="width:100%">Ответить</button></form><div class="inline-actions"><button class="compact-button" data-ticket-status="${id}:open">Открыть</button><button class="compact-button" data-ticket-status="${id}:closed">Закрыть</button></div>`)}

document.addEventListener('click',async event=>{const status=event.target.closest('[data-ticket-status]');if(!status)return;const [id,value]=status.dataset.ticketStatus.split(':');try{await api(`/api/admin/tickets/${id}`,{method:'PATCH',body:JSON.stringify({status:value})});state.cache['admin:tickets']=await api('/api/admin/tickets');closeModal();toast('Статус изменён');render()}catch(e){toast(e.message,true)}});
document.addEventListener('keydown', event => { if(event.key==='Escape'&&$('#modal-root').children.length)closeModal(); });

async function loadBootstrap() { state.data = await api('/api/bootstrap'); applyTheme(state.data.theme); render(); }

async function init() {
  try {
    if (state.preview) { state.data = previewData(); applyTheme(state.data.theme); render(); return; }
    if (!state.token) {
      if (!tg?.initData) throw new Error('Откройте приложение кнопкой внутри Telegram-бота.');
      const auth = await api('/api/auth/telegram', {method:'POST', body:JSON.stringify({init_data:tg.initData, referral_code:tg.initDataUnsafe?.start_param || ''})});
      state.token=auth.token;sessionStorage.setItem('tgs_session',state.token);
    }
    await loadBootstrap();
    if (new URLSearchParams(location.search).get('page') === 'trial' && !state.data.user.trial_used) setTimeout(()=>document.querySelector('[data-action="trial"]')?.click(),250);
  } catch(e) { sessionStorage.removeItem('tgs_session'); $('#app').innerHTML=`<main class="locked-screen">${icon('link')}<h1>TGS VPN</h1><p>${esc(e.message)}</p></main>`; }
}

function previewData(){return {user:{id:1,telegram_id:6402520205,username:'bruh',first_name:'Максим',photo_url:'',is_admin:true,trial_used:false,subscription:{status:'ACTIVE',expires_at:new Date(Date.now()+37*86400000).toISOString(),traffic_limit_bytes:107374182400,traffic_used_bytes:28991029248,device_limit:3,connected_devices:2,subscription_url:'https://example.com/sub'}},content:{brand:'TGS VPN',start_title:'Добро пожаловать',start_text:'',trial_button:'Бесплатный период',connect_button:'Подключиться',renew_button:'Продлить',support_welcome:'Опишите вопрос — поддержка ответит в этом чате.',emergency_message:'Сервис временно недоступен.'},features:{trial:true,server_status:true,devices:true,payments:true,referrals:true,promo_codes:true,support:true},trial:{enabled:true,days:3,traffic_gb:10,device_limit:1},theme:{template:'telegram',accent:'#2aabee',background:'#111315',surface:'#1c1f22',surface_alt:'#24282d',text:'#ffffff',muted:'#8f969e'},language:{default:'ru'},emergency:{enabled:false},more_order:['servers','devices','payments','referral'],tariffs:[{id:'1',name:'Старт',description:'Для одного устройства',price_rub:199,days:30,traffic_gb:100,device_limit:1,pinned:false},{id:'2',name:'Оптимальный',description:'Три месяца без забот',price_rub:499,days:90,traffic_gb:300,device_limit:3,pinned:true},{id:'3',name:'Годовой',description:'Максимальная выгода',price_rub:1490,days:365,traffic_gb:0,device_limit:5,pinned:false}],tickets:[{id:'t1',subject:'Не подключается на iPhone',status:'answered',created_at:new Date().toISOString(),updated_at:new Date().toISOString(),messages:[{text:'Не получается добавить подписку',is_admin:false,created_at:new Date().toISOString()},{text:'Проверьте разрешение VPN в настройках iOS.',is_admin:true,created_at:new Date().toISOString()}]}],payments:[{id:'p1',provider:'yookassa',status:'succeeded',amount_rub:499,snapshot:{name:'Оптимальный'},created_at:new Date().toISOString()}],referral:{count:4,link:'https://t.me/rwTGS_bot?start=tgs17d4b22',referral_days:7,referral_traffic_gb:10},payment_methods:[{id:'yookassa',name:'ЮKassa',enabled:true},{id:'cryptobot',name:'CryptoBot',enabled:true}]};}
async function previewApi(path,options){await new Promise(r=>setTimeout(r,140));if(path==='/api/nodes')return [{name:'Германия',country_code:'DE',status:'online'},{name:'Нидерланды',country_code:'NL',status:'online'},{name:'Финляндия',country_code:'FI',status:'offline'}];if(path==='/api/devices')return [{hwid:'iphone-demo',deviceModel:'iPhone 16 Pro',platform:'iOS',userAgent:'Happ 3.1'},{hwid:'windows-demo',deviceModel:'Windows PC',platform:'Windows',userAgent:'Hiddify'}];if(path.includes('/overview'))return {users:128,active_subscriptions:94,revenue_kopecks:18423000,diagnostics:2};if(path.includes('/settings/')){const key=path.split('/').pop();const value={language:state.data.language,emergency:state.data.emergency,features:state.data.features,trial:state.data.trial,theme:state.data.theme,more_order:{items:state.data.more_order},system:state.data.referral,content:state.data.content,integrations:{remnawave:{enabled:false,url:'https://panel.example.com',token:'••••••••'},yookassa:{enabled:false,shop_id:'',secret_key:'',email:''},cryptobot:{enabled:false,token:'',testnet:false},notifications:{enabled:false,bot_token:'',chat_id:''}}}[key];return {key,value,theme_templates:{telegram:{accent:'#2aabee',background:'#111315',surface:'#1c1f22',surface_alt:'#24282d'},graphite:{accent:'#ffffff',background:'#0c0c0d',surface:'#19191b',surface_alt:'#232326'},emerald:{accent:'#2fbf8f',background:'#101413',surface:'#1a211f',surface_alt:'#222c29'},sand:{accent:'#e7b55e',background:'#14120f',surface:'#211e19',surface_alt:'#2b271f'}}};}if(path.includes('/tariffs'))return state.data.tariffs.map(x=>({...x,active:true,internal_squads:[],external_squad_uuid:'',position:0}));if(path.includes('/users'))return [state.data.user];if(path.includes('/tickets'))return state.data.tickets.map(x=>({...x,user:state.data.user}));if(path.includes('/diagnostics'))return [{id:'d1',source:'remnawave',message:'Проверка подключения не пройдена',resolved:false,created_at:new Date().toISOString()}];if(path.includes('/promos'))return [{id:'pr1',code:'WELCOME',discount_percent:10,uses:7,max_uses:100,active:true}];return {ok:true,message:'Готово'};}

init();
