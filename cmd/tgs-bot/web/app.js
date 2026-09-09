const tg = window.Telegram?.WebApp;
tg?.ready();
tg?.expand();
tg?.disableClosingConfirmation?.();

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const esc = (value = '') => String(value).replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
const isPreview = new URLSearchParams(location.search).get('preview') === '1';

const i18n = {
  ru: {home:'Главная', tariffs:'Тарифы', support:'Поддержка', more:'Разное', admin:'Админ', expires:'Подписка до', noSubscription:'Нет подписки', devices:'Устройства', traffic:'Трафик', renew:'Продлить', connect:'Подключиться', buy:'Оплатить', days:'дней', unlimited:'Безлимит', createTicket:'Создать тикет', tickets:'Ваши тикеты', serverStatus:'Статус серверов', payments:'Платежи', referral:'Реферальная система'},
  en: {home:'Home', tariffs:'Plans', support:'Support', more:'More', admin:'Admin', expires:'Subscription until', noSubscription:'No subscription', devices:'Devices', traffic:'Traffic', renew:'Renew', connect:'Connect', buy:'Pay', days:'days', unlimited:'Unlimited', createTicket:'Create ticket', tickets:'Your tickets', serverStatus:'Server status', payments:'Payments', referral:'Referral program'}
};

const params = new URLSearchParams(location.search);
const state = { token: sessionStorage.getItem('tgs_session') || '', data: null, page: params.get('page') || 'home', preview: isPreview, cache: {}, selectedTariffID: '' };
const tr = key => i18n[state.data?.language?.default || 'ru']?.[key] || i18n.ru[key] || key;

const icon = name => `<svg class="icon${name === 'loader' ? ' icon-loader' : ''}" viewBox="0 0 24 24" aria-hidden="true"><use href="/assets/icons.svg#icon-${name}"></use></svg>`;

const platform = params.get('platform') || tg?.platform || '';
const hasNativeBack = ['android', 'ios'].includes(platform);
function parentPage() {
  if (state.page.startsWith('ticket:')) return 'support';
  if (state.page.startsWith('more:')) return 'more';
  if (state.page.startsWith('admin:')) return 'admin';
  return 'home';
}
function isDetailPage() { return state.page.includes(':'); }
function navigate(page) {
  if (page === state.page) return;
  state.page = page;
  state.cache = {...state.cache, overview:state.cache.overview};
  history.replaceState(null, '', `/app?page=${encodeURIComponent(state.page)}${state.preview ? '&preview=1' : ''}${hasNativeBack ? `&platform=${platform}` : ''}`);
  tg?.HapticFeedback?.selectionChanged?.();
  render();
}
function syncBackButton() {
  if (!tg?.BackButton) return;
  if (hasNativeBack && isDetailPage()) tg.BackButton.show();
  else tg.BackButton.hide();
}
tg?.BackButton?.onClick?.(() => navigate(parentPage()));
function backControl(target) {
  return hasNativeBack ? '' : `<button class="back-button" data-nav="${target}" aria-label="Назад">${icon('back')}</button>`;
}
function detailHead(title, target) { return `<header class="detail-head">${backControl(target)}<h2>${esc(title)}</h2></header>`; }
function loadingState(label = 'Загрузка') { return `<div class="loading-state">${icon('loader')}<span>${esc(label)}</span></div>`; }

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
function formatStamp(value) { if (!value) return ''; return new Intl.DateTimeFormat(state.data?.language?.default === 'en' ? 'en-GB' : 'ru-RU', {day:'2-digit', month:'2-digit', hour:'2-digit', minute:'2-digit'}).format(new Date(value)); }
function bytes(value) { if (!value) return '0 ГБ'; return `${(value / 1073741824).toLocaleString('ru-RU', {maximumFractionDigits:1})} ГБ`; }
function money(value) { return `${Number(value || 0).toLocaleString('ru-RU')} ₽`; }
function statusLabel(status) { return ({ACTIVE:'Активна',INACTIVE:'Не активна',EXPIRED:'Истекла',DISABLED:'Отключена',LIMITED:'Лимит',open:'Открыт',answered:'Ответ получен',closed:'Закрыт',pending:'Ожидает',processing:'Обработка',succeeded:'Оплачен',failed:'Ошибка'}[status] || status); }

function navigationItems() {
  const items = [
    ['home','home',tr('home')], ['tariffs','plans',tr('tariffs')], ['support','support',tr('support')], ['more','more',tr('more')]
  ];
  if (state.data.user.is_admin) items.push(['admin','admin',tr('admin')]);
  return items;
}

function navigationRootPage() {
  if (state.page.startsWith('ticket:')) return 'support';
  if (state.page.startsWith('more:')) return 'more';
  if (state.page.startsWith('admin:')) return 'admin';
  return state.page;
}

function nav() {
  const items = navigationItems();
  const rootPage = navigationRootPage();
  const activeIndex = Math.max(0, items.findIndex(([page]) => page === rootPage));
  return `<nav class="bottom-nav" data-nav-keys="${items.map(([page]) => page).join(',')}" style="--nav-count:${items.length};--indicator-x:${activeIndex * 100}%;--nav-width:${items.length * 48 + 16}px" aria-label="Основная навигация"><span class="nav-indicator" aria-hidden="true"></span>${items.map(([page,ico,label]) => `<button class="nav-button ${rootPage === page ? 'active' : ''}" data-nav="${page}" aria-label="${esc(label)}" title="${esc(label)}" ${rootPage === page ? 'aria-current="page"' : ''}>${icon(ico)}</button>`).join('')}</nav>`;
}

let renderedPage = '';
function ensureShell() {
  if ($('.app-shell', $('#app'))) return;
  $('#app').innerHTML = `<div class="app-shell"><main class="page" id="page-view"></main><div id="nav-slot">${nav()}</div></div>`;
}

function updateNavigation() {
  const items = navigationItems();
  const keys = items.map(([page]) => page).join(',');
  const slot = $('#nav-slot');
  let navigation = $('.bottom-nav', slot);
  if (!navigation || navigation.dataset.navKeys !== keys) {
    slot.innerHTML = nav();
    navigation = $('.bottom-nav', slot);
  }
  const rootPage = navigationRootPage();
  const activeIndex = Math.max(0, items.findIndex(([page]) => page === rootPage));
  navigation.style.setProperty('--indicator-x', `${activeIndex * 100}%`);
  $$('.nav-button', navigation).forEach(button => {
    const active = button.dataset.nav === rootPage;
    button.classList.toggle('active', active);
    if (active) button.setAttribute('aria-current', 'page');
    else button.removeAttribute('aria-current');
  });
}

function render() {
  if (!state.data) return;
  applyTheme(state.data.theme);
  if (state.data.emergency?.enabled && !state.data.user.is_admin) {
    $('#app').innerHTML = `<main class="locked-screen">${icon('alert')}<h1>Технические работы</h1><p>${esc(state.data.emergency.message||state.data.content.emergency_message)}</p></main>`;
    tg?.BackButton?.hide?.();
    renderedPage = '';
    return;
  }
  let body;
  if (state.page.startsWith('ticket:')) body = ticketPage(state.page.split(':')[1]);
  else if (state.page.startsWith('more:')) body = moreDetail(state.page.split(':')[1]);
  else if (state.page.startsWith('admin:')) body = adminDetail(state.page.split(':')[1]);
  else body = ({home:homePage, tariffs:tariffsPage, support:supportPage, more:morePage, admin:adminPage}[state.page] || homePage)();
  ensureShell();
  updateNavigation();
  const view = $('#page-view');
  const viewPage = state.page.split(':')[0];
  const commit = () => {
    view.className = `page page-${viewPage}`;
    view.innerHTML = body;
  };
  const pageChanged = Boolean(renderedPage && renderedPage !== state.page);
  commit();
  if (pageChanged) view.scrollTop = 0;
  if (pageChanged && !matchMedia('(prefers-reduced-motion: reduce)').matches) {
    view.getAnimations().forEach(animation => animation.cancel());
    view.animate([{opacity:.9, transform:'translateY(3px)'}, {opacity:1, transform:'translateY(0)'}], {duration:140, easing:'cubic-bezier(.32,.72,0,1)'});
  }
  renderedPage = state.page;
  syncBackButton();
  if (state.page.startsWith('ticket:')) requestAnimationFrame(() => { const chat = $('.chat'); if (chat) chat.scrollTop = chat.scrollHeight; });
}

function homePage() {
  const {user, trial, features, content} = state.data;
  const sub = user.subscription;
  const expires = sub.expires_at ? new Date(sub.expires_at) : null;
  const expired = Boolean(expires && expires <= new Date()) || ['EXPIRED', 'DISABLED'].includes(sub.status);
  const active = sub.status === 'ACTIVE' && !expired;
  const hasSubscription = active || Boolean(sub.subscription_url || sub.expires_at);
  const limit = Number(sub.traffic_limit_bytes || 0), used = Number(sub.traffic_used_bytes || 0);
  const progress = limit ? Math.min(100, used / limit * 100) : 0;
  if (!hasSubscription) {
    const trialButton = features.trial && trial.enabled && !user.trial_used ? `<button class="secondary" data-action="trial">${icon('gift')}<span>${esc(content.trial_button || 'Бесплатный период')}</span></button>` : '';
    return `<div class="home-stage"><section class="hero subscription-empty"><span class="status-dot danger" aria-label="Подписки нет"></span><div class="empty-state-icon">${icon('unavailable')}</div><h2>Подписки нету</h2><div class="compact-stack">${trialButton}<button class="primary" data-nav="tariffs">${icon('plans')}<span>Купить подписку</span></button></div></section></div>`;
  }
  if (expired || !active) {
    return `<div class="home-stage"><section class="hero subscription-empty"><span class="status-dot danger" aria-label="Подписка закончилась"></span><div class="empty-state-icon">${icon('unavailable')}</div><h2>Подписка закончилась</h2><div class="compact-stack"><button class="primary" data-nav="tariffs">${icon('plans')}<span>${esc(content.renew_button || tr('renew'))}</span></button></div></section></div>`;
  }
  return `<div class="home-stage"><section class="hero subscription-active">
    <div class="hero-head"><div><p class="eyebrow">${tr('expires')}</p><h2 class="expiry">${formatDate(sub.expires_at)}</h2></div><span class="status-dot active" aria-label="Подписка активна"></span></div>
    <div class="hero-stats"><div class="stat-block"><small>${tr('devices')}</small><strong>${Number(sub.connected_devices || 0)} / ${sub.device_limit || 0}</strong></div><div class="stat-block"><small>${tr('traffic')}</small><strong>${limit ? bytes(limit) : tr('unlimited')}</strong></div></div>
    <div class="traffic-row"><span>Использовано</span><strong>${bytes(used)}${limit ? ` из ${bytes(limit)}` : ''}</strong></div><div class="progress" role="progressbar" aria-valuenow="${Math.round(progress)}" aria-valuemin="0" aria-valuemax="100"><i style="width:${progress}%"></i></div>
    <div class="compact-stack"><button class="secondary" data-nav="tariffs">${icon('plans')}<span>${esc(content.renew_button || tr('renew'))}</span></button><button class="primary" data-action="connect" ${sub.subscription_url ? '' : 'disabled'}>${icon('link')}<span>${esc(content.connect_button || tr('connect'))}</span></button></div>
  </section></div>`;
}

function row(ico,title,subtitle,target,action='nav') { return `<button class="list-row" data-${action}="${esc(target)}"><span class="icon-box">${icon(ico)}</span><span><strong>${esc(title)}</strong><small>${esc(subtitle)}</small></span><span class="chevron">${icon('arrow')}</span></button>`; }

function tariffsPage() {
  const tariffs = state.data.tariffs || [];
  if (!tariffs.length) return empty('plans','Тарифы пока не добавлены');
  if (!tariffs.some(t => t.id === state.selectedTariffID)) state.selectedTariffID = (tariffs.find(t => t.pinned) || tariffs[0]).id;
  const selected = tariffs.find(t => t.id === state.selectedTariffID);
  return `<div class="tariffs-stage"><div class="tariff-list">${tariffs.map(t => { const active=t.id===state.selectedTariffID; return `<button type="button" class="tariff ${active ? 'selected' : ''}" data-select-tariff="${t.id}" aria-pressed="${active}">${t.pinned ? '<span class="tariff-badge">Выгодно</span>' : ''}<h3>${esc(t.name)}</h3><p>${esc(t.description)}</p><div class="tariff-meta"><span>${icon('clock')}${t.days} ${tr('days')}</span><span>${icon('devices')}${t.device_limit} устр.</span><span>${icon('traffic')}${t.traffic_gb ? `${t.traffic_gb} ГБ` : tr('unlimited')}</span></div><div class="tariff-footer"><span class="price">${money(t.price_rub)}</span><span class="tariff-selection">${active ? `${icon('check')}Выбрано` : ''}</span></div></button>`; }).join('')}</div><button class="primary tariff-pay" data-buy="${selected.id}"><span>${tr('buy')} · ${money(selected.price_rub)}</span></button></div>`;
}

function supportPage() {
  const tickets = state.data.tickets || [];
  return `<div class="support-intro"><p>${esc(state.data.content.support_welcome)}</p><button class="primary" data-action="new-ticket">${icon('plus')}<span>${tr('createTicket')}</span></button></div><div class="section-title"><h3>${tr('tickets')}</h3><span>${tickets.length}</span></div><div class="list ticket-list">${tickets.length ? tickets.map(t => { const last=t.messages?.[t.messages.length-1]; return `<button class="list-row ticket-row" data-nav="ticket:${t.id}"><span class="icon-box">${icon('support')}</span><span class="ticket-copy"><strong>${esc(t.subject)}</strong><small>${esc(last?.text || 'Сообщений пока нет')}</small></span><span class="ticket-side"><span class="ticket-time">${formatStamp(t.updated_at)}</span><span class="status ${esc(t.status)}">${statusLabel(t.status)}</span></span></button>`; }).join('') : empty('support','Открытых тикетов нет')}</div>`;
}

function ticketMessage(message) {
  return `<article class="bubble ${message.is_admin ? 'support-message' : 'mine'}"><strong>${message.is_admin ? 'Поддержка' : 'Вы'}</strong><p>${esc(message.text)}</p><time>${formatStamp(message.created_at)}</time></article>`;
}

function ticketMessages(messages) {
  return messages.length ? messages.map(ticketMessage).join('') : empty('support','Сообщений пока нет');
}

function ticketPage(id) {
  const ticket = (state.page.startsWith('admin:') ? state.cache.adminTickets : state.data.tickets)?.find(t => t.id === id);
  if (!ticket) return `${detailHead('Тикет', 'support')}${empty('support','Тикет не найден')}`;
  const messages = ticket.messages || [];
  return `<section class="ticket-layout">${detailHead(ticket.subject, 'support')}<div class="ticket-context"><span class="status ${esc(ticket.status)}">${statusLabel(ticket.status)}</span></div><div class="chat" aria-label="Переписка с поддержкой">${ticketMessages(messages)}</div>${ticket.status !== 'closed' ? `<form class="chat-compose" id="chat-form"><input name="text" maxlength="5000" aria-label="Сообщение" placeholder="Сообщение…" autocomplete="off"><button class="icon-button" aria-label="Отправить">${icon('send')}</button></form>` : '<p class="notice">Тикет закрыт</p>'}</section>`;
}

const moreItems = {
  servers:['server','Статус серверов','Доступность нод Remnawave'], devices:['devices','Устройства','Управление подключениями'], payments:['card','Платежи','История покупок'], referral:['referral','Реферальная система','Ссылка и вознаграждения']
};
function morePage() {
  const order = state.data.more_order || Object.keys(moreItems), f = state.data.features;
  const enabled = {servers:f.server_status, devices:f.devices, payments:f.payments, referral:f.referrals};
  const panelUsername = state.data.user.remnawave_username || `tgs_${state.data.user.telegram_id}`;
  return `<section class="profile-plain">${avatar(state.data.user,'profile-avatar')}<strong>${esc(panelUsername)}</strong></section><div class="list">${order.filter(key => enabled[key] && moreItems[key]).map(key => row(moreItems[key][0],moreItems[key][1],moreItems[key][2],`more:${key}`)).join('')}</div>`;
}

function moreDetail(key) {
  const labels = moreItems[key] || ['info','Раздел',''];
  const head = detailHead(labels[1], 'more');
  if (key === 'payments') return `${head}<div class="list">${state.data.payments?.length ? state.data.payments.map(p => `<div class="list-row" style="cursor:default"><span class="icon-box">${icon('card')}</span><span><strong>${money(p.amount_rub)}</strong><small>${esc(p.snapshot?.name || p.provider)} · ${formatDate(p.created_at)}</small></span><span class="status ${p.status}">${statusLabel(p.status)}</span></div>`).join('') : empty('card','Платежей пока нет')}</div>`;
  if (key === 'referral') { const ref = state.data.referral; return `${head}<section class="hero"><p class="eyebrow">Приглашено</p><h2 class="expiry">${ref.count}</h2><div class="hero-stats"><div class="stat-block"><small>Бонус</small><strong>${ref.referral_days} дней</strong></div><div class="stat-block"><small>Трафик</small><strong>${ref.referral_traffic_gb} ГБ</strong></div></div><div class="code">${esc(ref.link)}</div><button class="primary full-width top-gap" data-copy="${esc(ref.link)}">${icon('copy')}Скопировать ссылку</button></section>`; }
  const cached = state.cache[key];
  if (!cached) { loadMore(key); return `${head}${loadingState()}`; }
  if (key === 'servers') return `${head}<div class="list">${cached.length ? cached.map(n => `<div class="list-row" style="cursor:default"><span class="icon-box">${icon('server')}</span><span><strong>${esc(n.name || 'Node')}</strong><small>${esc(n.country_code || 'Локация не указана')}</small></span><span class="status ${n.status}">${n.status === 'online' ? 'Работает' : 'Недоступен'}</span></div>`).join('') : empty('server','Ноды не найдены')}</div>`;
  if (key === 'devices') return `${head}<div class="list">${cached.length ? cached.map((d,idx) => `<div class="list-row"><span class="icon-box">${icon('devices')}</span><span><strong>${esc(d.deviceModel || d.platform || `Устройство ${idx+1}`)}</strong><small>${esc(d.userAgent || d.hwid || 'HWID')}</small></span><button class="compact-button danger icon-only" data-delete-device="${esc(d.hwid || '')}" aria-label="Удалить устройство">${icon('trash')}</button></div>`).join('') : empty('devices','Подключённых устройств нет')}</div>`;
  return '';
}

async function loadMore(key) { try { state.cache[key] = await api(key === 'servers' ? '/api/nodes' : '/api/devices'); render(); } catch (e) { state.cache[key] = []; toast(e.message, true); render(); } }

function adminPage() {
  const items = [
    ['globe','Язык','Язык интерфейса','language'], ['alert','Режим аварии','Отключить доступ пользователям','emergency'], ['pulse','Диагностика','Ошибки панели и платежей','diagnostics'], ['toggles','Управление функциями','Включение разделов','features'], ['gift','Триал','Срок, трафик и сквады','trial'], ['link','Интеграции','Remnawave и платежи','integrations'], ['edit','Редактор контента','Тексты и брендинг','content'], ['palette','Оформление','Цветовые шаблоны','theme'], ['sort','Разное','Порядок пунктов','more_order'], ['plans','Тарифы','Каталог и сквады','tariffs'], ['user','Пользователи','Поиск и управление','users'], ['support','Тикеты поддержки','Ответы пользователям','tickets'], ['gear','Система','Реферальные бонусы','system'], ['megaphone','Рассылка','Создание через Telegram','broadcast'], ['percent','Промокоды','Скидки и лимиты','promos']
  ];
  if (!state.cache.overview) loadAdmin('overview');
  const o = state.cache.overview;
  return `${o ? `<div class="admin-metrics"><div class="metric"><span>Пользователи</span><strong>${o.users}</strong></div><div class="metric"><span>Активные</span><strong>${o.active_subscriptions}</strong></div><div class="metric"><span>Выручка</span><strong>${money((o.revenue_kopecks || 0)/100)}</strong></div><div class="metric"><span>Ошибки</span><strong>${o.diagnostics}</strong></div></div>` : loadingState()}<div class="list">${items.map(([ico,title,sub,key]) => row(ico,title,sub,`admin:${key}`)).join('')}</div>`;
}

async function loadAdmin(key, path = `/api/admin/${key}`) { try { state.cache[key] = await api(path); render(); } catch (e) { state.cache[key] = {error:e.message}; toast(e.message,true); render(); } }
function adminBack(title, subtitle='') { return `${detailHead(title, 'admin')}${subtitle ? `<p class="detail-subtitle">${esc(subtitle)}</p>` : ''}`; }

function adminDetail(key) {
  const settingKeys = ['language','emergency','features','trial','integrations','content','theme','more_order','system'];
  if (settingKeys.includes(key)) {
    if (!state.cache[`setting:${key}`]) { loadAdminSetting(key); return `${adminBack('Настройки')}${loadingState()}`; }
    return settingPage(key, state.cache[`setting:${key}`]);
  }
  if (key === 'diagnostics') return adminCollection(key,'Диагностика','/api/admin/diagnostics',diagnosticsView);
  if (key === 'tariffs') return adminCollection(key,'Тарифы','/api/admin/tariffs',adminTariffsView);
  if (key === 'users') return adminCollection(key,'Пользователи','/api/admin/users',adminUsersView);
  if (key === 'tickets') return adminCollection(key,'Тикеты поддержки','/api/admin/tickets',adminTicketsView);
  if (key === 'promos') return adminCollection(key,'Промокоды','/api/admin/promos',adminPromosView);
  if (key === 'broadcast') return adminCollection(key,'Рассылка','/api/admin/broadcast',broadcastView);
  return adminBack('Раздел');
}

async function ensureSquads() {
  if (state.cache.squads) return state.cache.squads;
  try { state.cache.squads = await api('/api/admin/squads'); }
  catch(e) { state.cache.squads={internal:[],external:[],error:e.message};toast(e.message,true); }
  return state.cache.squads;
}
async function loadAdminSetting(key) { try { const tasks=[api(`/api/admin/settings/${key}`)];if(key==='trial')tasks.push(ensureSquads());const [setting]=await Promise.all(tasks);state.cache[`setting:${key}`]=setting;render(); } catch(e) { toast(e.message,true); } }
function adminCollection(key,title,path,view) { if (!state.cache[`admin:${key}`]) { loadAdmin(`admin:${key}`,path); return `${adminBack(title)}${loadingState()}`; } const data=state.cache[`admin:${key}`]; return adminBack(title)+view(data); }

function switchRow(name,label,checked,description='') { return `<div class="switch-row"><span><strong>${esc(label)}</strong>${description ? `<small style="display:block;color:var(--muted);margin-top:3px">${esc(description)}</small>` : ''}</span><label class="switch"><input type="checkbox" name="${esc(name)}" ${checked ? 'checked' : ''}><i></i></label></div>`; }
function field(name,label,value='',type='text',hint='',wide=true) { return `<div class="field ${wide ? 'wide' : ''}"><label for="f-${esc(name)}">${esc(label)}</label><input id="f-${esc(name)}" name="${esc(name)}" type="${type}" value="${esc(value)}">${hint ? `<p class="field-hint">${esc(hint)}</p>` : ''}</div>`; }
function textareaField(name,label,value='',hint='') { return `<div class="field wide"><label for="f-${esc(name)}">${esc(label)}</label><textarea id="f-${esc(name)}" name="${esc(name)}">${esc(value)}</textarea>${hint?`<p class="field-hint">${esc(hint)}</p>`:''}</div>`; }
function colorField(name,label,value) { return `<label class="color-field"><span>${esc(label)}</span><input type="color" name="${esc(name)}" value="${esc(value)}" aria-label="${esc(label)}"><code>${esc(value)}</code></label>`; }

function squadPicker(selectedInternal=[], selectedExternal='', prefix='squads') {
  const squads=state.cache.squads||{internal:[],external:[]}, selected=new Set(selectedInternal||[]), internal=squads.internal||[], external=squads.external||[];
  const preserved=squads.error?[...selected].map((uuid,index)=>`<input id="${prefix}-preserved-${index}" type="checkbox" name="internal_squads" value="${esc(uuid)}" data-array-field checked hidden>`).join(''):'';
  const choices=internal.length?internal.map((s,index)=>`<label class="squad-choice" for="${prefix}-squad-${index}"><input id="${prefix}-squad-${index}" type="checkbox" name="internal_squads" value="${esc(s.uuid)}" data-array-field ${selected.has(s.uuid)?'checked':''}><span>${esc(s.name)}</span></label>`).join(''):`${preserved}<p class="picker-empty">${esc(squads.error||'Внутренние сквады не найдены')}</p>`;
  const currentExternal=squads.error&&selectedExternal?`<option value="${esc(selectedExternal)}" selected>Текущий внешний сквад</option>`:'';
  return `<section class="squad-picker"><header><span><strong>Внутренние сквады</strong><small>Доступные серверы тарифа</small></span><span class="picker-actions"><button type="button" class="text-button" data-squads-all ${squads.error?'disabled':''}>Все</button><button type="button" class="text-button" data-squads-clear ${squads.error?'disabled':''}>Снять</button></span></header><div class="squad-options">${choices}</div><div class="field"><label for="${prefix}-external">Внешний сквад</label><select id="${prefix}-external" name="external_squad_uuid" ${squads.error?'disabled':''}><option value="">Не использовать</option>${currentExternal}${external.map(s=>`<option value="${esc(s.uuid)}" ${s.uuid===selectedExternal?'selected':''}>${esc(s.name)}</option>`).join('')}</select>${squads.error?`<input type="hidden" name="external_squad_uuid" value="${esc(selectedExternal)}">`:''}</div></section>`;
}

function settingsAccordion(title,subtitle,ico,content,open=false) { return `<details class="settings-accordion" ${open?'open':''}><summary><span class="icon-box">${icon(ico)}</span><span><strong>${esc(title)}</strong><small>${esc(subtitle)}</small></span><span class="accordion-arrow">${icon('arrow')}</span></summary><div class="accordion-body">${content}</div></details>`; }

function settingPage(key, payload) {
  const v = payload.value || {}, titles = {language:'Язык',emergency:'Режим аварии',features:'Управление функциями',trial:'Триал',integrations:'Интеграции',content:'Редактор контента',theme:'Оформление',more_order:'Разное',system:'Система'};
  let inner='';
  if (key === 'language') inner = `<div class="field"><label>Язык всего интерфейса</label><select name="default"><option value="ru" ${v.default==='ru'?'selected':''}>Русский</option><option value="en" ${v.default==='en'?'selected':''}>English</option></select></div>`;
  if (key === 'emergency') inner = switchRow('enabled','Аварийный режим',v.enabled,'Пользователи увидят только сообщение о работах') + textareaField('message','Сообщение пользователям',v.message||state.data.content.emergency_message||'Сервис временно недоступен.');
  if (key === 'features') inner = Object.entries({trial:'Бесплатный период',server_status:'Статус серверов',devices:'Устройства',payments:'Платежи',referrals:'Реферальная система',promo_codes:'Промокоды',support:'Поддержка'}).map(([n,l])=>switchRow(n,l,v[n])).join('');
  if (key === 'trial') inner = `<div class="form-grid">${switchRow('enabled','Триал включён',v.enabled)}${field('days','Количество дней',v.days,'number','',false)}${field('traffic_gb','Трафик, ГБ (0 = безлимит)',v.traffic_gb,'number','',false)}${field('device_limit','Устройств',v.device_limit,'number','',false)}</div>${squadPicker(v.internal_squads,v.external_squad_uuid,'trial')}`;
  if (key === 'content') return `${adminBack(titles[key])}${contentForms(v)}`;
  if (key === 'system') inner = `<section class="settings-group"><header><strong>Реферальная система</strong><small>Бонус владельцу ссылки после приглашения</small></header><div class="form-grid">${field('referral_days','Бонус дней',v.referral_days,'number','',false)}${field('referral_traffic_gb','Бонус трафика, ГБ',v.referral_traffic_gb,'number','',false)}${switchRow('reward_after_payment','Только после первой оплаты',v.reward_after_payment,'Без оплаты приглашённого бонус не начисляется')}</div></section>`;
  if (key === 'more_order') { const items=v.items||[]; inner=`<p class="notice">Изменяйте порядок кнопками. Видимость разделов задаётся в управлении функциями.</p><div class="list" id="order-list">${items.map((name,idx)=>`<div class="list-row" data-order-item="${name}"><span class="icon-box">${icon(moreItems[name]?.[0]||'info')}</span><span><strong>${esc(moreItems[name]?.[1]||name)}</strong></span><span class="order-actions"><button type="button" class="compact-button icon-only" data-move="${idx}:up" aria-label="Выше">${icon('arrow-up')}</button><button type="button" class="compact-button icon-only" data-move="${idx}:down" aria-label="Ниже">${icon('arrow-down')}</button></span></div>`).join('')}</div><input type="hidden" name="items" value="${esc(items.join(','))}">`; }
  if (key === 'theme') { const templates=payload.theme_templates||{},names={telegram:'Telegram',graphite:'Графит',emerald:'Изумруд',sand:'Песок',ocean:'Океан',violet:'Фиолетовый',ruby:'Рубин',steel:'Сталь'}; inner=`<div class="theme-grid">${Object.entries(templates).map(([name,t])=>`<button type="button" class="theme-option ${v.template===name?'active':''}" data-theme="${name}" data-theme-values="${esc(JSON.stringify(t))}"><span class="swatches"><i style="background:${t.background}"></i><i style="background:${t.surface}"></i><i style="background:${t.accent}"></i></span><strong>${esc(names[name]||name)}</strong></button>`).join('')}</div><input type="hidden" name="template" value="${esc(v.template||'telegram')}"><div class="color-grid">${colorField('accent','Акцент',v.accent)}${colorField('background','Фон',v.background)}${colorField('surface','Карточки',v.surface)}${colorField('surface_alt','Поля',v.surface_alt)}${colorField('text','Текст',v.text)}${colorField('muted','Вторичный текст',v.muted)}</div>`; }
  if (key === 'integrations') return `${adminBack(titles[key])}${integrationForms(v)}`;
  return `${adminBack(titles[key])}<form class="admin-form" data-setting-form="${key}">${inner}<button class="primary" style="width:100%;margin-top:16px">Сохранить</button></form>`;
}

function integrationForms(v) {
  const groups=[
    ['remnawave','Remnawave',[['url','URL панели','url'],['token','API Token','password'],['webhook_secret','Секрет webhook','password']]],
    ['yookassa','ЮKassa',[['shop_id','Shop ID','text'],['secret_key','Secret Key','password'],['email','Email для чеков','email']]],
    ['cryptobot','CryptoBot',[['token','API Token','password']]],
    ['notifications','Бот уведомлений',[['bot_token','Токен бота','password'],['chat_id','Chat ID','text']]]
  ];
  const icons={remnawave:'link',yookassa:'card',cryptobot:'link',notifications:'send'};
  return `<div class="accordion-stack">${groups.map(([key,title,fields],index)=>{const c=v[key]||{};const webhookHint=key==='remnawave'?'<p class="field-hint">Webhook: /api/webhooks/remnawave · заголовок X-Webhook-Secret</p>':'';const body=`<form data-setting-form="integrations">${switchRow(`${key}.enabled`,'Включено',c.enabled)}${fields.map(([n,l,t])=>field(`${key}.${n}`,l,c[n]||'',t)).join('')}${webhookHint}${key==='cryptobot'?switchRow('cryptobot.testnet','Тестовая сеть',c.testnet):''}<div class="form-actions">${key!=='notifications'?`<button type="button" class="secondary" data-test-integration="${key}">Проверить</button>`:''}<button class="primary">Сохранить</button></div></form>`;return settingsAccordion(title,c.enabled?'Подключено':'Выключено',icons[key],body,index===0)}).join('')}</div>`;
}

function contentForms(v) {
  const groups=[
    ['miniapp','Mini App','Кнопки и тексты приложения','home',[['connect_button','Кнопка подключения'],['renew_button','Кнопка продления'],['support_welcome','Текст поддержки']]],
    ['chat','Чат и /start','Приветствие и кнопки бота','support',[['start_title','Заголовок /start'],['start_text','Текст /start'],['trial_button','Кнопка триала'],['cabinet_button','Кнопка кабинета'],['support_button','Кнопка поддержки']]],
    ['brand','Брендинг','Название и логотип','palette',[['brand','Название'],['logo_url','URL логотипа']]]
  ];
  return `<div class="accordion-stack">${groups.map(([key,title,sub,ico,fields],index)=>settingsAccordion(title,sub,ico,`<form data-setting-form="content">${fields.map(([name,label])=>field(name,label,v[name]||'')).join('')}<button class="primary full-width">Сохранить</button></form>`,index===0)).join('')}</div>`;
}

function diagnosticsView(rows) { if (!Array.isArray(rows)) return empty('alert',rows.error||'Ошибка'); return `<div class="list">${rows.length ? rows.map(d=>`<div class="list-row"><span class="icon-box">${icon('pulse')}</span><span><strong>${esc(d.source)}</strong><small>${esc(d.message)} · ${formatDate(d.created_at)}</small></span>${d.resolved?'<span class="status active">Решено</span>':`<button class="compact-button" data-resolve="${d.id}">Решено</button>`}</div>`).join('') : empty('pulse','Ошибок нет')}</div>`; }
function adminTariffsView(rows) { if (!Array.isArray(rows)) return empty('plans','Ошибка'); return `<button class="primary" style="width:100%;margin-bottom:12px" data-admin-tariff="new">${icon('plus')}Создать тариф</button><div class="list">${rows.map(t=>`<button class="list-row" data-admin-tariff="${t.id}"><span class="icon-box">${icon('plans')}</span><span><strong>${esc(t.name)}</strong><small>${t.days} дней · ${money(t.price_rub)}${t.active?'':' · выключен'}</small></span>${icon('arrow')}</button>`).join('')}</div>`; }
function adminUsersView(rows) { if (!Array.isArray(rows)) return empty('user','Ошибка'); return `<form id="user-search" class="field" style="margin-top:0"><label>Поиск</label><input name="q" placeholder="Имя, username или Telegram ID"></form><div class="list">${rows.length?rows.map(u=>`<button class="list-row" data-admin-user="${u.id}">${avatar(u,'avatar')}<span><strong>${esc(u.first_name)}</strong><small>${u.username?'@'+esc(u.username):u.telegram_id} · ${statusLabel(u.subscription.status)}</small></span>${icon('arrow')}</button>`).join(''):empty('user','Пользователи не найдены')}</div>`; }
function adminTicketsView(rows) { if (!Array.isArray(rows)) return empty('support','Ошибка'); return `<div class="list">${rows.length?rows.map(t=>`<button class="list-row admin-ticket-row" data-admin-ticket="${t.id}"><span><strong>${esc(t.subject)}</strong><small>${esc(t.user?.first_name||'')} · ${formatDate(t.updated_at)}</small></span><span class="status ${t.status}">${statusLabel(t.status)}</span></button>`).join(''):empty('support','Тикетов нет')}</div>`; }
function adminPromosView(rows) { if (!Array.isArray(rows)) return empty('percent','Ошибка'); return `<button class="primary" style="width:100%;margin-bottom:12px" data-action="new-promo">${icon('plus')}Создать промокод</button><div class="list">${rows.length?rows.map(p=>`<div class="list-row"><span class="icon-box">${icon('percent')}</span><span><strong>${esc(p.code)} · ${p.discount_percent}%</strong><small>${p.uses}${p.max_uses?` / ${p.max_uses}`:''} использований</small></span><button class="compact-button danger" data-delete-promo="${p.id}" aria-label="Удалить">${icon('trash')}</button></div>`).join(''):empty('percent','Промокодов нет')}</div>`; }

function broadcastButtonRow(button={},index=0) {
  const input=(name,label,value,type='text',hint='')=>`<div class="field"><label for="broadcast-${index}-${name}">${label}</label><input id="broadcast-${index}-${name}" name="${name}" type="${type}" value="${esc(value)}">${hint?`<p class="field-hint">${hint}</p>`:''}</div>`;
  return `<section class="broadcast-button" data-broadcast-button><header><strong>Кнопка ${index+1}</strong><button type="button" class="text-button danger" data-remove-broadcast-button="${index}" aria-label="Удалить кнопку">${icon('trash')}</button></header><div class="form-grid">${input('button_text','Название',button.text||'')}${input('button_url','Ссылка',button.url||'','url')}<div class="field"><label for="broadcast-${index}-style">Цвет</label><select id="broadcast-${index}-style" name="button_style"><option value="" ${!button.style?'selected':''}>Обычная</option><option value="primary" ${button.style==='primary'?'selected':''}>Синяя</option><option value="success" ${button.style==='success'?'selected':''}>Зелёная</option><option value="danger" ${button.style==='danger'?'selected':''}>Красная</option></select></div>${input('button_emoji','ID премиум-эмодзи',button.icon_custom_emoji_id||'','text','Необязательно')}</div></section>`;
}

function broadcastView(payload) {
  const draft=payload?.draft, statusLabels={awaiting:'Ожидает сообщения',preview:'Нужно подтвердить',confirmed:'Готова',sending:'Отправляется',completed:'Завершена',failed:'Ошибка',cancelled:'Отменена'};
  const controls=`<div class="broadcast-top"><button class="primary" data-broadcast-draft>${icon('edit')}<span>${draft?'Изменить сообщение':'Создать сообщение'}</span></button><button class="secondary icon-only-action" data-broadcast-refresh aria-label="Обновить">${icon('pulse')}</button></div>`;
  if (!draft) return `${controls}${empty('megaphone','Сообщение для рассылки ещё не создано')}`;
  const ready=['confirmed','sending','completed'].includes(draft.status), buttons=Array.isArray(draft.buttons)?draft.buttons:[];
  return `${controls}<section class="broadcast-summary"><span class="status ${esc(draft.status)}">${esc(statusLabels[draft.status]||draft.status)}</span><strong>${esc(draft.text||'Отправьте сообщение боту')}</strong><small>${draft.source_message_id?'Оригинал сохранён в Telegram':'Перейдите в чат и отправьте одно сообщение'}</small></section>${ready?`<form id="broadcast-buttons-form" data-id="${draft.id}"><div id="broadcast-buttons">${buttons.map(broadcastButtonRow).join('')}</div><button type="button" class="secondary full-width" data-add-broadcast-button ${buttons.length>=8?'disabled':''}>${icon('plus')}Добавить кнопку</button><p class="field-hint broadcast-hint">Цвет и премиум-эмодзи поддерживаются Telegram не для всех ботов и клиентов.</p><div class="broadcast-actions"><button type="button" class="secondary" data-broadcast-test>Тест</button><button type="button" class="primary" data-broadcast-send ${draft.status==='sending'?'disabled':''}>Рассылка</button></div></form>`:`<p class="notice">После отправки нажмите «Подтвердить» под предпросмотром в Telegram, затем вернитесь сюда и обновите экран.</p>`}`;
}

function empty(ico,text) { return `<div class="empty">${icon(ico)}${esc(text)}</div>`; }

let modalCloseTimer = null;
let modalTrigger = null;
function openModal(title, content, actions = '') {
  if (modalCloseTimer) clearTimeout(modalCloseTimer);
  if (!$('.modal-backdrop')) modalTrigger = document.activeElement;
  $('#modal-root').innerHTML = `<div class="modal-backdrop" data-modal-backdrop><section class="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title"><header class="modal-head"><h3 id="modal-title">${esc(title)}</h3><button class="modal-close" data-close-modal aria-label="Закрыть">${icon('close')}</button></header>${content}${actions}</section></div>`;
  setTimeout(() => $('.modal input, .modal select, .modal button')?.focus(), 30);
}
function closeModal(immediate = false) {
  const root = $('#modal-root');
  const backdrop = $('.modal-backdrop', root);
  if (!backdrop) return;
  if (backdrop.classList.contains('closing')) return;
  if (immediate || matchMedia('(prefers-reduced-motion: reduce)').matches) {
    root.innerHTML = '';
    modalTrigger?.focus?.();
    return;
  }
  backdrop.classList.add('closing');
  backdrop.setAttribute('aria-hidden', 'true');
  modalCloseTimer = setTimeout(() => {
    if (root.firstElementChild === backdrop) root.innerHTML = '';
    modalTrigger?.focus?.();
    modalCloseTimer = null;
  }, 170);
}

function buyModal(id) {
  const tariff=state.data.tariffs.find(t=>t.id===id), methods=state.data.payment_methods.filter(m=>m.enabled);
  openModal(`Оплата — ${tariff.name}`, `<form id="checkout-form"><input type="hidden" name="tariff_id" value="${tariff.id}"><div class="choice-list">${methods.length?methods.map((m,i)=>`<label class="choice"><input type="radio" name="provider" value="${m.id}" ${i===0?'checked':''}><span><strong>${esc(m.name)}</strong><small style="display:block;color:var(--muted);margin-top:3px">${m.id==='yookassa'?'Банковская карта или СБП':'Криптовалюта через Telegram'}</small></span></label>`).join(''):empty('card','Способы оплаты ещё не подключены')}</div>${state.data.features.promo_codes?field('promo_code','Промокод','','text','Необязательно'):''}<button class="primary" style="width:100%;margin-top:15px" ${methods.length?'':'disabled'}>Оплатить ${money(tariff.price_rub)}</button></form>`);
}
function ticketModal() { openModal('Новый тикет', `<form id="ticket-form">${field('subject','Тема','','text','Коротко опишите вопрос')}<div class="field"><label for="ticket-description">Описание</label><textarea id="ticket-description" name="description" maxlength="5000" required placeholder="Что случилось?"></textarea></div><button class="primary" style="width:100%">Отправить</button></form>`); }

async function tariffAdminModal(id) { openModal(id==='new'?'Новый тариф':'Редактирование тарифа',loadingState('Загружаем сквады'));try{await ensureSquads();const existing=id==='new'?{name:'',description:'',price_rub:199,days:30,traffic_gb:100,device_limit:1,internal_squads:[],external_squad_uuid:'',active:true,pinned:false,position:0}:state.cache['admin:tariffs'].find(t=>t.id===id);openModal(id==='new'?'Новый тариф':'Редактирование тарифа',`<form id="tariff-admin-form" data-id="${id}"><div class="form-grid">${field('name','Название',existing.name)}${field('description','Описание',existing.description)}${field('price_rub','Цена, ₽',existing.price_rub,'number','',false)}${field('days','Дней',existing.days,'number','',false)}${field('traffic_gb','Трафик, ГБ (0 = безлимит)',existing.traffic_gb,'number','',false)}${field('device_limit','Устройств',existing.device_limit,'number','',false)}</div>${squadPicker(existing.internal_squads,existing.external_squad_uuid,`tariff-${id}`)}${switchRow('active','Тариф включён',existing.active)}${switchRow('pinned','Закрепить первым',existing.pinned)}${field('position','Позиция',existing.position,'number')}<div class="modal-actions">${id!=='new'?`<button type="button" class="danger-button" data-remove-tariff="${id}">Удалить</button>`:'<button type="button" class="secondary" data-close-modal>Отмена</button>'}<button class="primary">Сохранить</button></div></form>`);}catch(e){closeModal(true);toast(e.message,true)} }
function userAdminModal(id){const u=state.cache['admin:users'].find(x=>String(x.id)===String(id));openModal(u.first_name,`<form id="user-admin-form" data-id="${u.id}"><div class="code">Telegram ID: ${u.telegram_id}<br>${u.username?'@'+esc(u.username):'Без username'}<br>Подписка до: ${formatDate(u.subscription.expires_at)}<br>Рефералов: ${u.referral_count||0} · код ${esc(u.referral_code||'—')}</div><div class="field"><label>Статус VPN в Remnawave</label><select name="subscription_status"><option value="">Не менять</option><option value="ACTIVE">Включить VPN</option><option value="DISABLED">Отключить VPN</option></select></div><div class="form-grid">${field('add_days','Добавить дней',0,'number','',false)}${field('add_traffic_gb','Добавить ГБ',0,'number','',false)}${field('device_limit','Лимит устройств',u.subscription.device_limit,'number','',false)}</div>${switchRow('blocked','Заблокировать кабинет',u.is_blocked,'Не отключает VPN')}${switchRow('admin','Администратор',u.is_admin)}${field('reason','Причина изменения','','text','Сохранится в журнале действий')}<div class="modal-actions"><button type="button" class="danger-button" data-full-block-user>Полная блокировка</button><button class="primary">Сохранить</button></div></form>`)}
function promoModal(){openModal('Новый промокод',`<form id="promo-form">${field('code','Код','','text')}${field('discount_percent','Скидка, %',10,'number')}${field('max_uses','Лимит использований (0 = без лимита)',0,'number')}${switchRow('active','Активен',true)}<button class="primary" style="width:100%;margin-top:14px">Создать</button></form>`)}

function collectBroadcastButtons(form) { return $$('.broadcast-button',form).map(row=>({text:$('[name="button_text"]',row).value.trim(),url:$('[name="button_url"]',row).value.trim(),style:$('[name="button_style"]',row).value,icon_custom_emoji_id:$('[name="button_emoji"]',row).value.trim()})); }
async function saveBroadcastButtons(form) { const id=form.dataset.id,buttons=collectBroadcastButtons(form);const draft=await api(`/api/admin/broadcast/${id}/buttons`,{method:'PUT',body:JSON.stringify({buttons})});state.cache['admin:broadcast'].draft=draft;return draft; }

document.addEventListener('click', async event => {
  const navButton=event.target.closest('[data-nav]'); if(navButton){navigate(navButton.dataset.nav);return;}
  if(event.target.closest('[data-close-modal]') || (event.target.matches('[data-modal-backdrop]'))){closeModal();return;}
  const tariffChoice=event.target.closest('[data-select-tariff]');if(tariffChoice){const id=tariffChoice.dataset.selectTariff;if(id===state.selectedTariffID)return;state.selectedTariffID=id;$$('[data-select-tariff]').forEach(card=>{const selected=card.dataset.selectTariff===id;card.classList.toggle('selected',selected);card.setAttribute('aria-pressed',String(selected));const marker=$('.tariff-selection',card);if(marker)marker.innerHTML=selected?`${icon('check')}Выбрано`:''});const tariff=state.data.tariffs.find(item=>item.id===id);const pay=$('.tariff-pay');if(tariff&&pay){pay.dataset.buy=id;$('span',pay).textContent=`${tr('buy')} · ${money(tariff.price_rub)}`;}tg?.HapticFeedback?.selectionChanged?.();return;}
  const buy=event.target.closest('[data-buy]');if(buy){buyModal(buy.dataset.buy);return;}
  const action=event.target.closest('[data-action]')?.dataset.action;
  if(action==='new-ticket'){ticketModal();return;} if(action==='new-promo'){promoModal();return;}
  if(action==='trial'){const button=event.target.closest('[data-action="trial"]');const old=button?.innerHTML;if(button){button.disabled=true;button.innerHTML=`${icon('loader')}<span>Подключаем…</span>`}try{await api('/api/trial',{method:'POST',body:'{}'});await loadBootstrap();toast('Пробный период активирован');}catch(e){if(button){button.disabled=false;button.innerHTML=old}toast(e.message,true)}return;}
  if(action==='connect'){const link=state.data.user.subscription.subscription_url;if(link){tg?.openLink?tg.openLink(link):window.open(link,'_blank')}return;}
  const copy=event.target.closest('[data-copy]');if(copy){await navigator.clipboard.writeText(copy.dataset.copy);toast('Ссылка скопирована');return;}
  const device=event.target.closest('[data-delete-device]');if(device&&confirm('Удалить это устройство?')){try{await api(`/api/devices/${encodeURIComponent(device.dataset.deleteDevice)}`,{method:'DELETE'});state.cache.devices=null;loadMore('devices');toast('Устройство удалено');}catch(e){toast(e.message,true)}return;}
  const allSquads=event.target.closest('[data-squads-all]');if(allSquads){$$('[data-array-field]',allSquads.closest('.squad-picker')).forEach(input=>input.checked=true);return;}
  const clearSquads=event.target.closest('[data-squads-clear]');if(clearSquads){$$('[data-array-field]',clearSquads.closest('.squad-picker')).forEach(input=>input.checked=false);return;}
  const tariff=event.target.closest('[data-admin-tariff]');if(tariff){await tariffAdminModal(tariff.dataset.adminTariff);return;}
  const user=event.target.closest('[data-admin-user]');if(user){userAdminModal(user.dataset.adminUser);return;}
  const fullBlock=event.target.closest('[data-full-block-user]');if(fullBlock){if(!confirm('Заблокировать кабинет и отключить VPN?'))return;const form=fullBlock.closest('form');form.elements.blocked.checked=true;form.elements.subscription_status.value='DISABLED';form.requestSubmit();return;}
  const ticket=event.target.closest('[data-admin-ticket]');if(ticket){adminTicketModal(ticket.dataset.adminTicket);return;}
  if(event.target.closest('[data-remove-tariff]')){const id=event.target.closest('[data-remove-tariff]').dataset.removeTariff;if(confirm('Удалить тариф?')){await api(`/api/admin/tariffs/${id}`,{method:'DELETE'});closeModal();state.cache['admin:tariffs']=null;loadAdmin('admin:tariffs','/api/admin/tariffs');toast('Тариф удалён')}return;}
  const promo=event.target.closest('[data-delete-promo]');if(promo&&confirm('Удалить промокод?')){await api(`/api/admin/promos/${promo.dataset.deletePromo}`,{method:'DELETE'});state.cache['admin:promos']=null;loadAdmin('admin:promos','/api/admin/promos');toast('Промокод удалён');return;}
  const resolve=event.target.closest('[data-resolve]');if(resolve){await api(`/api/admin/diagnostics/${resolve.dataset.resolve}`,{method:'PATCH',body:'{}'});state.cache['admin:diagnostics']=null;loadAdmin('admin:diagnostics','/api/admin/diagnostics');return;}
  const test=event.target.closest('[data-test-integration]');if(test){try{const value=await api(`/api/admin/integrations/${test.dataset.testIntegration}/test`,{method:'POST',body:'{}'});toast(value.message)}catch(e){toast(e.message,true)}return;}
  const newBroadcast=event.target.closest('[data-broadcast-draft]');if(newBroadcast){try{newBroadcast.disabled=true;const out=await api('/api/admin/broadcast/draft',{method:'POST',body:'{}'});state.cache['admin:broadcast']=out;render();if(tg?.openTelegramLink)tg.openTelegramLink(out.bot_url);else window.open(out.bot_url,'_blank');}catch(e){newBroadcast.disabled=false;toast(e.message,true)}return;}
  if(event.target.closest('[data-broadcast-refresh]')){try{state.cache['admin:broadcast']=await api('/api/admin/broadcast');render();toast('Обновлено')}catch(e){toast(e.message,true)}return;}
  if(event.target.closest('[data-add-broadcast-button]')){const draft=state.cache['admin:broadcast']?.draft;if(draft&&draft.buttons.length<8){draft.buttons.push({text:'',url:'',style:'',icon_custom_emoji_id:''});render()}return;}
  const removeBroadcast=event.target.closest('[data-remove-broadcast-button]');if(removeBroadcast){const draft=state.cache['admin:broadcast']?.draft;if(draft){draft.buttons.splice(Number(removeBroadcast.dataset.removeBroadcastButton),1);render()}return;}
  const broadcastTest=event.target.closest('[data-broadcast-test]');if(broadcastTest){try{const form=broadcastTest.closest('form');await saveBroadcastButtons(form);await api(`/api/admin/broadcast/${form.dataset.id}/test`,{method:'POST',body:'{}'});toast('Тест отправлен вам')}catch(e){toast(e.message,true)}return;}
  const broadcastSend=event.target.closest('[data-broadcast-send]');if(broadcastSend){if(!confirm('Отправить сообщение всем пользователям?'))return;try{const form=broadcastSend.closest('form');await saveBroadcastButtons(form);await api(`/api/admin/broadcast/${form.dataset.id}/send`,{method:'POST',body:'{}'});state.cache['admin:broadcast'].draft.status='sending';render();toast('Рассылка запущена')}catch(e){toast(e.message,true)}return;}
  const theme=event.target.closest('[data-theme]');if(theme){const form=theme.closest('form');const values=JSON.parse(theme.dataset.themeValues);form.elements.template.value=theme.dataset.theme;Object.entries(values).forEach(([k,v])=>{if(form.elements[k]){form.elements[k].value=v;form.elements[k].closest('.color-field')?.querySelector('code')?.replaceChildren(v)}});applyTheme({...state.data.theme,...values});$$('.theme-option',form).forEach(x=>x.classList.toggle('active',x===theme));return;}
  const move=event.target.closest('[data-move]');if(move){const [idxRaw,dir]=move.dataset.move.split(':');const input=move.closest('form').elements.items;const items=input.value.split(',');const idx=Number(idxRaw),next=dir==='up'?idx-1:idx+1;if(next>=0&&next<items.length){[items[idx],items[next]]=[items[next],items[idx]];input.value=items.join(',');state.cache['setting:more_order'].value.items=items;render()}return;}
});

document.addEventListener('submit', async event => {
  event.preventDefault(); const form=event.target;
  try {
    if(form.id==='checkout-form'){const body=Object.fromEntries(new FormData(form));const out=await api('/api/payments/checkout',{method:'POST',body:JSON.stringify(body)});closeModal();tg?.openLink?tg.openLink(out.pay_url):window.open(out.pay_url,'_blank');toast(`Счёт на ${money(out.amount_rub)} создан`);return;}
    if(form.id==='ticket-form'){const body=Object.fromEntries(new FormData(form));const ticket=await api('/api/tickets',{method:'POST',body:JSON.stringify(body)});state.data.tickets.unshift(ticket);closeModal();navigate(`ticket:${ticket.id}`);toast('Тикет создан');return;}
    if(form.id==='chat-form'){const input=form.elements.text,draft=input.value.trim();if(!draft)return;const id=state.page.split(':')[1],idx=state.data.tickets.findIndex(t=>t.id===id),oldCount=Math.max(0,state.data.tickets[idx]?.messages?.length||0),button=$('button',form),buttonHTML=button.innerHTML;input.disabled=true;button.disabled=true;button.innerHTML=icon('loader');try{const ticket=await api(`/api/tickets/${id}/messages`,{method:'POST',body:JSON.stringify({text:draft})});state.data.tickets[idx]=ticket;const chat=$('.chat'),messages=ticket.messages||[];if(chat){if(messages.length>=oldCount&&oldCount>0){messages.slice(oldCount).forEach(message=>chat.insertAdjacentHTML('beforeend',ticketMessage(message)))}else{chat.innerHTML=ticketMessages(messages)}chat.scrollTo({top:chat.scrollHeight,behavior:matchMedia('(prefers-reduced-motion: reduce)').matches?'auto':'smooth'})}const status=$('.ticket-context .status');if(status){status.className=`status ${ticket.status}`;status.textContent=statusLabel(ticket.status)}input.value='';}finally{if(document.contains(form)){input.disabled=false;button.disabled=false;button.innerHTML=buttonHTML;input.focus()}}return;}
    if(form.dataset.settingForm){const key=form.dataset.settingForm,body=collectForm(form);const out=await api(`/api/admin/settings/${key}`,{method:'PUT',body:JSON.stringify({value:body})});state.cache[`setting:${key}`]=out;if(key==='theme'){state.data.theme={...state.data.theme,...out.value};applyTheme(state.data.theme)}if(key==='language')state.data.language=out.value;if(key==='emergency')state.data.emergency=out.value;if(key==='features')state.data.features={...state.data.features,...out.value};if(key==='content')state.data.content={...state.data.content,...out.value};if(key==='more_order')state.data.more_order=out.value.items;toast('Сохранено');if(!['integrations','content'].includes(key))render();return;}
    if(form.id==='tariff-admin-form'){const id=form.dataset.id,body=collectForm(form);body.price_rub=Number(body.price_rub);['days','traffic_gb','device_limit','position'].forEach(k=>body[k]=Number(body[k]));const method=id==='new'?'POST':'PUT',path=id==='new'?'/api/admin/tariffs':`/api/admin/tariffs/${id}`;await api(path,{method,body:JSON.stringify(body)});closeModal();state.cache['admin:tariffs']=null;await loadAdmin('admin:tariffs','/api/admin/tariffs');toast('Тариф сохранён');return;}
    if(form.id==='user-admin-form'){const body=collectForm(form);['add_days','add_traffic_gb','device_limit'].forEach(k=>body[k]=Number(body[k]));await api(`/api/admin/users/${form.dataset.id}`,{method:'PATCH',body:JSON.stringify(body)});closeModal();state.cache['admin:users']=null;await loadAdmin('admin:users','/api/admin/users');toast('Пользователь обновлён');return;}
    if(form.id==='promo-form'){const body=collectForm(form);body.discount_percent=Number(body.discount_percent);body.max_uses=Number(body.max_uses);await api('/api/admin/promos',{method:'POST',body:JSON.stringify(body)});closeModal();state.cache['admin:promos']=null;await loadAdmin('admin:promos','/api/admin/promos');toast('Промокод создан');return;}
    if(form.id==='user-search'){state.cache['admin:users']=null;await loadAdmin('admin:users',`/api/admin/users?q=${encodeURIComponent(form.elements.q.value)}`);return;}
    if(form.id==='admin-ticket-chat'){const id=form.dataset.id,textValue=form.elements.text.value.trim();if(!textValue)return;await api(`/api/tickets/${id}/messages`,{method:'POST',body:JSON.stringify({text:textValue})});state.cache['admin:tickets']=await api('/api/admin/tickets');closeModal();adminTicketModal(id);return;}
  } catch(e) { toast(e.message,true); }
});

function collectForm(form) { const out={};$$('[name]',form).forEach(input=>{const path=input.name.split('.');let cursor=out;path.slice(0,-1).forEach(part=>cursor=cursor[part]||(cursor[part]={}));const key=path[path.length-1];if(input.hasAttribute('data-array-field')){if(!Array.isArray(cursor[key]))cursor[key]=[];if(input.checked)cursor[key].push(input.value);return;}let value=input.type==='checkbox'?input.checked:input.value;if(input.type==='number')value=Number(value);cursor[key]=value;});if(form.dataset.settingForm==='more_order')out.items=(out.items||'').split(',').filter(Boolean);return out;}

function adminTicketModal(id){const ticket=state.cache['admin:tickets'].find(t=>t.id===id);openModal(ticket.subject,`<div class="chat" style="padding-bottom:8px;max-height:48vh;overflow:auto">${ticket.messages.map(m=>`<div class="bubble ${m.is_admin?'mine':''}">${esc(m.text)}<small>${formatDate(m.created_at)}</small></div>`).join('')}</div><form id="admin-ticket-chat" data-id="${id}"><div class="field"><label>Ответ</label><textarea name="text" maxlength="5000"></textarea></div><button class="primary" style="width:100%">Ответить</button></form><div class="inline-actions"><button class="compact-button" data-ticket-status="${id}:open">Открыть</button><button class="compact-button" data-ticket-status="${id}:closed">Закрыть</button></div>`)}

document.addEventListener('click',async event=>{const status=event.target.closest('[data-ticket-status]');if(!status)return;const [id,value]=status.dataset.ticketStatus.split(':');try{await api(`/api/admin/tickets/${id}`,{method:'PATCH',body:JSON.stringify({status:value})});state.cache['admin:tickets']=await api('/api/admin/tickets');closeModal();toast('Статус изменён');render()}catch(e){toast(e.message,true)}});
document.addEventListener('keydown', event => { if(event.key==='Escape'&&$('#modal-root').children.length)closeModal(); });
document.addEventListener('input', event => { if(event.target.matches('input[type="color"]')){event.target.closest('.color-field')?.querySelector('code')?.replaceChildren(event.target.value);const form=event.target.closest('form');if(form)applyTheme({...state.data.theme,...collectForm(form)})} });

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

function previewData(){return {user:{id:1,telegram_id:6402520205,username:'bruh',remnawave_username:'tgs_6402520205',first_name:'Максим',photo_url:'',is_admin:true,trial_used:false,subscription:{status:'ACTIVE',expires_at:new Date(Date.now()+37*86400000).toISOString(),traffic_limit_bytes:107374182400,traffic_used_bytes:28991029248,device_limit:3,connected_devices:2,subscription_url:'https://example.com/sub'}},content:{brand:'TGS VPN',start_title:'Добро пожаловать',start_text:'',trial_button:'Бесплатный период',connect_button:'Подключиться',renew_button:'Продлить',support_welcome:'Опишите вопрос — поддержка ответит в этом чате.',emergency_message:'Сервис временно недоступен.'},features:{trial:true,server_status:true,devices:true,payments:true,referrals:true,promo_codes:true,support:true},trial:{enabled:true,days:3,traffic_gb:10,device_limit:1,internal_squads:[],external_squad_uuid:''},theme:{template:'telegram',accent:'#2aabee',background:'#111315',surface:'#1c1f22',surface_alt:'#24282d',text:'#ffffff',muted:'#8f969e'},language:{default:'ru'},emergency:{enabled:false,message:'Сервис временно недоступен.'},more_order:['servers','devices','payments','referral'],tariffs:[{id:'1',name:'Старт',description:'Для одного устройства',price_rub:199,days:30,traffic_gb:100,device_limit:1,pinned:false},{id:'2',name:'Оптимальный',description:'Три месяца без забот',price_rub:499,days:90,traffic_gb:300,device_limit:3,pinned:true},{id:'3',name:'Годовой',description:'Максимальная выгода',price_rub:1490,days:365,traffic_gb:0,device_limit:5,pinned:false}],tickets:[{id:'t1',subject:'Не подключается на iPhone',status:'answered',created_at:new Date().toISOString(),updated_at:new Date().toISOString(),messages:[{text:'Не получается добавить подписку',is_admin:false,created_at:new Date(Date.now()-3600000).toISOString()},{text:'Проверьте разрешение VPN в настройках iOS.',is_admin:true,created_at:new Date().toISOString()}]}],payments:[{id:'p1',provider:'yookassa',status:'succeeded',amount_rub:499,snapshot:{name:'Оптимальный'},created_at:new Date().toISOString()}],referral:{count:4,link:'https://t.me/rwTGS_bot?start=tgs17d4b22',referral_days:7,referral_traffic_gb:10},payment_methods:[{id:'yookassa',name:'ЮKassa',enabled:true},{id:'cryptobot',name:'CryptoBot',enabled:true}]};}
async function previewApi(path,options={}) {
  await new Promise(resolve=>setTimeout(resolve,80));
  if(path==='/api/nodes')return [{name:'Германия',country_code:'DE',status:'online'},{name:'Нидерланды',country_code:'NL',status:'online'},{name:'Финляндия',country_code:'FI',status:'offline'}];
  if(path==='/api/devices')return [{hwid:'iphone-demo',deviceModel:'iPhone 16 Pro',platform:'iOS',userAgent:'Happ 3.1'},{hwid:'windows-demo',deviceModel:'Windows PC',platform:'Windows',userAgent:'Hiddify'}];
  if(path==='/api/admin/squads')return {internal:[{uuid:'de-main',name:'Германия · Основной'},{uuid:'nl-fast',name:'Нидерланды · Быстрый'}],external:[{uuid:'public',name:'Публичный сквад'}]};
  if(path.includes('/overview'))return {users:128,active_subscriptions:94,revenue_kopecks:18423000,diagnostics:2};
  if(path.includes('/settings/')){const key=path.split('/').pop();const value={language:state.data.language,emergency:state.data.emergency,features:state.data.features,trial:state.data.trial,theme:state.data.theme,more_order:{items:state.data.more_order},system:state.data.referral,content:state.data.content,integrations:{remnawave:{enabled:false,url:'https://panel.example.com',token:'••••••••'},yookassa:{enabled:false,shop_id:'',secret_key:'',email:''},cryptobot:{enabled:false,token:'',testnet:false},notifications:{enabled:false,bot_token:'',chat_id:''}}}[key];return {key,value,theme_templates:{telegram:{accent:'#2aabee',background:'#111315',surface:'#1c1f22',surface_alt:'#24282d'},graphite:{accent:'#ffffff',background:'#0c0c0d',surface:'#19191b',surface_alt:'#232326'},emerald:{accent:'#2fbf8f',background:'#101413',surface:'#1a211f',surface_alt:'#222c29'},sand:{accent:'#e7b55e',background:'#14120f',surface:'#211e19',surface_alt:'#2b271f'},ocean:{accent:'#5aa9e6',background:'#0d1217',surface:'#182028',surface_alt:'#222d36'},violet:{accent:'#9b8cff',background:'#111016',surface:'#1d1a25',surface_alt:'#282333'},ruby:{accent:'#e06b78',background:'#151012',surface:'#22191c',surface_alt:'#2e2226'},steel:{accent:'#8ea2b5',background:'#101214',surface:'#1a1e22',surface_alt:'#242a30'}}};}
  if(path==='/api/admin/broadcast')return state.cache['admin:broadcast']||{draft:{id:'demo',text:'Новое сообщение пользователям',source_message_id:15,status:'confirmed',buttons:[]},bot_url:'https://t.me/rwTGS_bot'};
  if(path.includes('/broadcast/draft'))return {draft:{id:'demo',text:'',source_message_id:0,status:'awaiting',buttons:[]},bot_url:'https://t.me/rwTGS_bot'};
  if(path.includes('/tariffs'))return state.data.tariffs.map(x=>({...x,active:true,internal_squads:[],external_squad_uuid:'',position:0}));
  if(path.includes('/users'))return [state.data.user];
  if(path.includes('/tickets'))return state.data.tickets.map(x=>({...x,user:state.data.user}));
  if(path.includes('/diagnostics'))return [{id:'d1',source:'remnawave',message:'Проверка подключения не пройдена',resolved:false,created_at:new Date().toISOString()}];
  if(path.includes('/promos'))return [{id:'pr1',code:'WELCOME',discount_percent:10,uses:7,max_uses:100,active:true}];
  return {ok:true,message:'Готово'};
}

init();
