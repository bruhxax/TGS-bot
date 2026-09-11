const tg = window.Telegram?.WebApp;
tg?.ready();
tg?.expand();
tg?.disableClosingConfirmation?.();

const $ = (selector, root = document) => root.querySelector(selector);
const $$ = (selector, root = document) => [...root.querySelectorAll(selector)];
const esc = (value = '') => String(value).replace(/[&<>'"]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;',"'":'&#39;','"':'&quot;'}[c]));
const mergeObjects=(base,update)=>{const out={...(base||{})};Object.entries(update||{}).forEach(([key,value])=>{out[key]=value&&typeof value==='object'&&!Array.isArray(value)?mergeObjects(out[key],value):value});return out};
const isPreview = new URLSearchParams(location.search).get('preview') === '1';

const i18n = {
  ru: {home:'Главная', tariffs:'Тарифы', support:'Поддержка', more:'Разное', admin:'Админ', expires:'Подписка до', noSubscription:'Нет подписки', devices:'Устройства', traffic:'Трафик', renew:'Продлить', connect:'Подключиться', buy:'Оплатить', days:'дней', unlimited:'Безлимит', createTicket:'Создать тикет', tickets:'Ваши тикеты', serverStatus:'Статус серверов', payments:'Платежи', referral:'Реферальная система'},
  en: {home:'Home', tariffs:'Plans', support:'Support', more:'More', admin:'Admin', expires:'Subscription until', noSubscription:'No subscription', devices:'Devices', traffic:'Traffic', renew:'Renew', connect:'Connect', buy:'Pay', days:'days', unlimited:'Unlimited', createTicket:'Create ticket', tickets:'Your tickets', serverStatus:'Server status', payments:'Payments', referral:'Referral program'}
};

const params = new URLSearchParams(location.search);
const state = { token: sessionStorage.getItem('tgs_session') || '', data: null, page: params.get('page') || 'home', preview: isPreview, cache: {}, selectedTariffID: '', scrollPositions: {}, supportLoading: false };
const tr = key => i18n[state.data?.language?.default || 'ru']?.[key] || i18n.ru[key] || key;

let iconSpritePrefix = '/assets/icons.svg?v=20260910-v104';
const icon = name => `<svg class="icon${name === 'loader' ? ' icon-loader' : ''}" viewBox="0 0 24 24" aria-hidden="true"><use href="${iconSpritePrefix}#icon-${name}"></use></svg>`;
const paymentLogo = provider => { const ext=['yookassa','cryptobot'].includes(provider)?'jpg':'png'; return `<span class="payment-logo ${esc(provider)}" aria-hidden="true"><img src="/assets/payment-${esc(provider)}.${ext}?v=20260910-v104" alt=""></span>`; };

async function loadIconSprite() {
  try {
    const response = await fetch('/assets/icons.svg?v=20260910-v104', {cache:'force-cache'});
    if (!response.ok) return;
    const parsed = new DOMParser().parseFromString(await response.text(), 'image/svg+xml');
    if (parsed.querySelector('parsererror') || !parsed.querySelector('symbol')) return;
    const sprite = document.importNode(parsed.documentElement, true);
    sprite.id = 'icon-sprite';
    sprite.classList.add('icon-sprite');
    sprite.setAttribute('aria-hidden', 'true');
    document.body.prepend(sprite);
    iconSpritePrefix = '';
  } catch (_) {
    // The external sprite remains as a safe fallback.
  }
}

const platform = params.get('platform') || tg?.platform || '';
const hasNativeBack = ['android', 'ios'].includes(platform);
function parentPage() {
	if (state.page === 'connect') return 'home';
  if (state.page.startsWith('ticket:')) return 'support';
  if (state.page.startsWith('more:')) return 'more';
  if (state.page.startsWith('admin:')) return 'admin';
  return 'home';
}
function isDetailPage() { return state.page === 'connect' || state.page.includes(':'); }
function rememberScroll() {
  const view = $('#page-view');
  if (view) state.scrollPositions[state.page] = view.scrollTop;
}
function navigate(page) {
  if (page === state.page) return;
  rememberScroll();
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
function formatExpiryDay(value) { if (!value) return ''; return new Intl.DateTimeFormat(state.data?.language?.default === 'en' ? 'en-GB' : 'ru-RU', {day:'numeric', month:'long'}).format(new Date(value)); }
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
	if (state.page === 'connect') return 'home';
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
  const currentView = $('#page-view');
  if (currentView && renderedPage === state.page) state.scrollPositions[state.page] = currentView.scrollTop;
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
  else body = ({home:homePage,connect:connectPage, tariffs:tariffsPage, support:supportPage, more:morePage, admin:adminPage}[state.page] || homePage)();
  ensureShell();
  updateNavigation();
  const view = $('#page-view');
  const viewPage = state.page.split(':')[0];
  const targetPage = state.page;
  const savedScroll = state.scrollPositions[targetPage] || 0;
  const commit = () => {
    view.className = `page page-${viewPage}`;
    view.innerHTML = body;
  };
  const pageChanged = Boolean(renderedPage && renderedPage !== state.page);
  commit();
  view.scrollTop = savedScroll;
  requestAnimationFrame(() => {
    if (state.page === targetPage) view.scrollTop = savedScroll;
  });
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
  const expiryDay = formatExpiryDay(sub.expires_at), expiryYear = expires?.getFullYear() || '';
  const logo=/^https:\/\//.test(content.logo_url||'')?`<img class="home-logo" src="${esc(content.logo_url)}" alt="">`:'';
  const frame = card => `<div class="home-stage"><div class="home-content"><div class="home-branding">${logo}<p class="home-brand">${esc(content.brand || 'TGS VPN')}</p></div>${card}</div></div>`;
  if (!hasSubscription) {
    const trialButton = features.trial && trial.enabled && !user.trial_used ? `<button class="secondary" data-action="trial">${icon('gift')}<span>${esc(content.trial_button || 'Бесплатный период')}</span></button>` : '';
    return frame(`<section class="hero subscription-empty"><span class="status-dot danger" aria-label="Подписки нет"></span><div class="empty-state-icon">${icon('unavailable')}</div><h2>${esc(content.no_subscription_title||'Подписки нету')}</h2><div class="compact-stack">${trialButton}<button class="primary" data-nav="tariffs">${icon('plans')}<span>${esc(content.buy_subscription_button||'Купить подписку')}</span></button></div></section>`);
  }
  if (expired || !active) {
    return frame(`<section class="hero subscription-empty"><span class="status-dot danger" aria-label="Подписка закончилась"></span><div class="empty-state-icon">${icon('unavailable')}</div><h2>${esc(content.subscription_expired_title||'Подписка закончилась')}</h2><div class="compact-stack"><button class="primary" data-nav="tariffs">${icon('plans')}<span>${esc(content.renew_button || tr('renew'))}</span></button></div></section>`);
  }
  return frame(`<section class="hero subscription-active">
    <div class="subscription-period"><span class="subscription-period-icon">${icon('clock')}</span><div><p class="eyebrow">${tr('expires')}</p><h2 class="expiry-date"><span>${esc(expiryDay)}</span><small>${esc(expiryYear)}</small></h2></div></div><span class="status-dot active" aria-label="Подписка активна"></span>
    <div class="subscription-facts"><div class="subscription-fact"><span class="subscription-fact-icon">${icon('devices')}</span><span><small>${tr('devices')}</small><strong>${Number(sub.connected_devices || 0)} / ${Number(sub.device_limit) > 0 ? Number(sub.device_limit) : '∞'}</strong></span></div><div class="subscription-fact"><span class="subscription-fact-icon">${icon('traffic')}</span><span><small>${tr('traffic')}</small><strong>${limit ? bytes(limit) : tr('unlimited')}</strong></span></div></div>
    <div class="traffic-row"><span>${esc(content.traffic_used_label||'Использовано')}</span><strong>${bytes(used)}${limit ? ` из ${bytes(limit)}` : ''}</strong></div><div class="progress" role="progressbar" aria-valuenow="${Math.round(progress)}" aria-valuemin="0" aria-valuemax="100"><i style="width:${progress}%"></i></div>
    <div class="compact-stack"><button class="secondary" data-nav="tariffs">${icon('plans')}<span>${esc(content.renew_button || tr('renew'))}</span></button><button class="primary" data-action="connect" ${sub.subscription_url ? '' : 'disabled'}>${icon('link')}<span>${esc(content.connect_button || tr('connect'))}</span></button></div>
  </section>`);
}

const setupPlatformMeta = [
  ['ios','iOS'],['android','Android'],['macos','macOS'],['windows','Windows'],['android-tv','Android TV'],['apple-tv','Apple TV']
];
const builtinSetupClients = {
  ios:[{id:'happ',name:'Happ',scheme:'happ://add/',install_url:'https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973',featured:true},{id:'incy',name:'INCY',scheme:'incy://import/',install_url:'https://apps.apple.com/ru/app/incy/id6756943388'}],
  android:[{id:'happ',name:'Happ',scheme:'happ://add/',install_url:'https://play.google.com/store/apps/details?id=com.happproxy',featured:true},{id:'incy',name:'INCY',scheme:'incy://import/',install_url:'https://play.google.com/store/apps/details?id=llc.itdev.incy'}],
  macos:[{id:'happ',name:'Happ',scheme:'happ://add/',install_url:'https://apps.apple.com/ru/app/happ-proxy-utility-plus/id6746188973',featured:true},{id:'incy',name:'INCY',scheme:'incy://import/',install_url:'https://github.com/INCY-DEV/incy-platforms/releases/latest/download/incy-macos-arm64.dmg'}],
  windows:[{id:'happ',name:'Happ',scheme:'happ://add/',install_url:'https://github.com/Happ-proxy/happ-desktop/releases/latest/download/setup-Happ.x64.exe',featured:true},{id:'incy',name:'INCY',scheme:'incy://import/',install_url:'https://github.com/INCY-DEV/incy-platforms/releases/latest/download/incy-windows-setup.exe'}],
  'android-tv':[{id:'happ',name:'Happ',scheme:'happ://add/',install_url:'https://play.google.com/store/apps/details?id=com.happproxy',featured:true},{id:'incy',name:'INCY',scheme:'incy://import/',install_url:'https://play.google.com/store/apps/details?id=llc.itdev.incy'}],
  'apple-tv':[{id:'happ',name:'Happ',scheme:'happ://add/',install_url:'https://apps.apple.com/us/app/happ-proxy-utility-for-tv/id6748297274',featured:true},{id:'incy',name:'INCY',scheme:'incy://import/',install_url:'https://apps.apple.com/ru/app/incy/id6756943388'}]
};
function detectedSetupPlatform(){const ua=navigator.userAgent.toLowerCase(),tgPlatform=String(platform||'').toLowerCase();if(/appletv|apple tv/.test(ua))return'apple-tv';if(/android/.test(ua)&&/tv|aft|bravia|smart-tv|googletv/.test(ua))return'android-tv';if(/iphone|ipad|ipod/.test(ua)||tgPlatform==='ios')return'ios';if(/android/.test(ua)||tgPlatform==='android')return'android';if(/macintosh|mac os x/.test(ua)||tgPlatform==='macos')return'macos';return'windows'}
function setupClients(platformID){const settings=state.data.subpage||{},items=settings.include_builtins===false?[]:[...(builtinSetupClients[platformID]||[])];(settings.clients||[]).forEach(client=>{if(!client.enabled)return;if(!client.all_platforms&&!(client.platforms||[]).includes(platformID))return;if(items.some(item=>item.id===client.id))return;const normalized={...client};if(normalized.featured)items.unshift(normalized);else items.push(normalized)});return items}
function connectHandoffKey(platformID,client,url){return `${platformID}|${client?.id||''}|${client?.scheme||''}|${url}`}
async function loadConnectHandoff(platformID,client,url,force=false){
  if(!client||!url)return null;
  const key=connectHandoffKey(platformID,client,url),cache=state.cache.connectHandoffs||(state.cache.connectHandoffs={});
  if(cache[key]&&!force)return cache[key];
  if(state.cache.connectHandoffPending===key&&!force)return null;
  state.cache.connectHandoffPending=key;
  try{cache[key]=await api('/api/connect/handoff',{method:'POST',body:JSON.stringify({scheme:client.scheme,client_name:client.name})});}
  catch(error){cache[key]={error:error.message};}
  finally{if(state.cache.connectHandoffPending===key)state.cache.connectHandoffPending='';}
  if(state.page==='connect'&&state.cache.setupPlatform===platformID&&state.cache.setupClient===client.id)render();
  return cache[key];
}
function openBrowserLink(url){if(tg?.openLink)tg.openLink(url,{try_instant_view:false});else window.open(url,'_blank','noopener,noreferrer')}
function setSetupPlatformMenu(open){const menu=$('.setup-platform-menu'),trigger=$('[data-setup-platform-trigger]');if(!menu||!trigger)return;menu.hidden=!open;trigger.setAttribute('aria-expanded',String(open));trigger.classList.toggle('open',open)}
function connectPage(){
  const content=state.data.content,sub=state.data.user.subscription,url=sub.subscription_url||'';
  if(!state.cache.setupPlatform)state.cache.setupPlatform=detectedSetupPlatform();
  const selected=setupPlatformMeta.some(([id])=>id===state.cache.setupPlatform)?state.cache.setupPlatform:'windows',clients=setupClients(selected);
  const platforms=setupPlatformMeta.filter(([id])=>setupClients(id).length);
  if(!clients.some(client=>client.id===state.cache.setupClient))state.cache.setupClient=(clients.find(client=>client.featured)||clients[0]||{}).id||'';
  const client=clients.find(item=>item.id===state.cache.setupClient),key=connectHandoffKey(selected,client,url),handoff=(state.cache.connectHandoffs||{})[key];
  if(client&&url&&!handoff&&state.cache.connectHandoffPending!==key)setTimeout(()=>loadConnectHandoff(selected,client,url),0);
  const platformLabel=setupPlatformMeta.find(([id])=>id===selected)?.[1]||'Windows';
  const platformOptions=platforms.map(([id,label])=>`<button type="button" role="option" aria-selected="${id===selected}" class="${id===selected?'active':''}" data-setup-platform-option="${id}"><span>${esc(label)}</span>${id===selected?icon('check'):''}</button>`).join('');
  const clientTabs=clients.length?clients.map(item=>`<button type="button" role="tab" aria-selected="${item.id===client?.id}" class="${item.id===client?.id?'active':''}" data-setup-client="${esc(item.id)}">${esc(item.name)}${item.featured?`<small>${esc(content.connect_recommended||'Рекомендуем')}</small>`:''}</button>`).join(''):'';
  const qr=handoff?.qr_data_url?`<img src="${esc(handoff.qr_data_url)}" alt="QR-код для добавления подписки">`:handoff?.error?`<button type="button" class="setup-qr-error" data-retry-connect>${icon('pulse')}<span>Повторить загрузку</span></button>`:`<div class="setup-qr-loading">${icon('loader')}<span>Готовим QR-код</span></div>`;
  if(!client)return `${detailHead(content.connect_title||'Установка','home')}${empty('devices',content.connect_empty||'Для этого устройства клиенты пока не добавлены')}`;
  return `<header class="setup-page-head">${backControl('home')}<h1 class="sr-only">Подключение</h1><div class="setup-platform-control" data-platform-control><button type="button" class="setup-platform-trigger" data-setup-platform-trigger aria-haspopup="listbox" aria-controls="setup-platform-menu" aria-expanded="false">${icon('devices')}<span>${esc(platformLabel)}</span>${icon('arrow')}</button><div class="setup-platform-menu" id="setup-platform-menu" role="listbox" aria-label="Выберите устройство" hidden>${platformOptions}</div></div></header><div class="setup-client-tabs" role="tablist" aria-label="Приложение">${clientTabs}</div><section class="setup-guide"><article class="setup-step"><span class="setup-step-index">01</span><div><h3>${esc(content.connect_install_title||'Установите приложение')}</h3><p>${esc(content.connect_install_hint||'Скачайте клиент для своего устройства.')}</p>${client.install_url?`<a class="setup-inline-action" href="${esc(client.install_url)}" target="_blank" rel="noopener noreferrer"><span>${esc(content.connect_install_button||'Скачать')} ${esc(client.name)}</span>${icon('arrow')}</a>`:''}</div></article><article class="setup-step"><span class="setup-step-index">02</span><div><h3>${esc(content.connect_add_title||'Добавьте подписку')}</h3><p>${esc(content.connect_add_hint||'Браузер предложит открыть выбранное приложение.')}</p><button type="button" class="primary setup-add-button" data-open-connect-handoff ${url?'':'disabled'}>${icon('plus')}<span>${esc(content.connect_open_button||'Добавить подписку')}</span></button></div></article><article class="setup-step"><span class="setup-step-index">03</span><div><h3>${esc(content.connect_use_title||'Подключитесь')}</h3><p>${esc(content.connect_use_hint||'Выберите сервер в приложении и включите подключение.')}</p></div></article></section><section class="setup-transfer" aria-label="QR-код подписки"><div class="setup-qr-card">${qr}</div><button type="button" class="secondary setup-copy-button" data-copy="${esc(url)}" ${url?'':'disabled'}>${icon('copy')}<span>${esc(content.connect_copy_button||'Скопировать ссылку')}</span></button></section>`;
}

function row(ico,title,subtitle,target,action='nav') { return `<button class="list-row" data-${action}="${esc(target)}"><span class="icon-box">${icon(ico)}</span><span><strong>${esc(title)}</strong><small>${esc(subtitle)}</small></span><span class="chevron">${icon('arrow')}</span></button>`; }

function tariffsPage() {
  const tariffs = state.data.tariffs || [];
  if (!tariffs.length) return empty('plans','Тарифы пока не добавлены');
  if (!tariffs.some(t => t.id === state.selectedTariffID)) state.selectedTariffID = (tariffs.find(t => t.pinned) || tariffs[0]).id;
  const selected = tariffs.find(t => t.id === state.selectedTariffID);
  return `<div class="tariffs-stage"><div class="tariff-list">${tariffs.map(t => { const active=t.id===state.selectedTariffID; return `<button type="button" class="tariff ${active ? 'selected' : ''}" data-select-tariff="${t.id}" aria-pressed="${active}">${t.pinned ? '<span class="tariff-badge">Выгодно</span>' : ''}<h3>${esc(t.name)}</h3><p>${esc(t.description)}</p><div class="tariff-meta"><span>${icon('clock')}${t.days} ${tr('days')}</span><span>${icon('devices')}${Number(t.device_limit) > 0 ? t.device_limit : '∞'} устр.</span><span>${icon('traffic')}${t.traffic_gb ? `${t.traffic_gb} ГБ` : tr('unlimited')}</span></div><div class="tariff-footer"><span class="price">${money(t.price_rub)}</span><span class="tariff-selection">${active ? `${icon('check')}Выбрано` : ''}</span></div></button>`; }).join('')}</div><button class="primary tariff-pay" data-buy="${selected.id}"><span>${tr('buy')} · ${money(selected.price_rub)}</span></button></div>`;
}

function supportPage() {
  const isAdmin = state.data.user.is_admin;
  if (isAdmin && !Array.isArray(state.cache.adminTickets)) {
    loadSupportTickets();
    return loadingState('Загружаем обращения');
  }
  const tickets = visibleTickets();
  const content=state.data.content;
  const intro = isAdmin ? `<div class="support-admin-caption"><strong>Обращения пользователей</strong><small>${esc(content.support_admin_hint||'Новые сообщения появляются автоматически')}</small></div>` : `<div class="support-intro"><p>${esc(content.support_welcome)}</p><button class="primary" data-action="new-ticket">${icon('plus')}<span>${esc(content.support_create_button||tr('createTicket'))}</span></button></div>`;
  const title = isAdmin ? (content.support_admin_title||'Все тикеты') : (content.support_tickets_title||tr('tickets'));
  return `${intro}<div class="section-title"><h3>${esc(title)}</h3><span>${tickets.length}</span></div><div class="list ticket-list">${tickets.length ? tickets.map(ticketRow).join('') : empty('support',content.support_empty||'Открытых тикетов нет')}</div>`;
}

function visibleTickets() { return state.data.user.is_admin ? (state.cache.adminTickets || []) : (state.data.tickets || []); }
async function loadSupportTickets() {
  if (state.supportLoading) { state.supportRefreshQueued=true;return; }
  state.supportLoading=true;
  const view=$('#page-view'), chat=$('.chat'), input=$('#chat-form input');
  const snapshot={page:state.page,viewTop:view?.scrollTop||0,chatTop:chat?.scrollTop||0,nearBottom:chat?chat.scrollHeight-chat.scrollTop-chat.clientHeight<44:true,draft:input?.value||'',focused:document.activeElement===input};
  try {
    const rows=await api(state.data.user.is_admin?'/api/admin/tickets':'/api/tickets');
    if(state.data.user.is_admin)state.cache.adminTickets=rows;else state.data.tickets=rows;
    if(state.page==='support'||state.page.startsWith('ticket:')){
      render();
      requestAnimationFrame(()=>requestAnimationFrame(()=>{
        if(state.page!==snapshot.page)return;
        const nextView=$('#page-view'),nextChat=$('.chat'),nextInput=$('#chat-form input');
        if(nextView&&!state.page.startsWith('ticket:'))nextView.scrollTop=snapshot.viewTop;
        if(nextChat)nextChat.scrollTop=snapshot.nearBottom?nextChat.scrollHeight:snapshot.chatTop;
        if(nextInput){nextInput.value=snapshot.draft;if(snapshot.focused)nextInput.focus()}
      }));
    }
  } catch(e) { if(!state.cache.adminTickets&&state.data.user.is_admin)state.cache.adminTickets=[];toast(e.message,true);if(state.page==='support')render(); }
  finally { state.supportLoading=false;if(state.supportRefreshQueued){state.supportRefreshQueued=false;loadSupportTickets()} }
}
function replaceTicket(ticket) {
  const rows=visibleTickets(), index=rows.findIndex(item=>item.id===ticket.id), previous=index>=0?rows[index]:null;
  if(!ticket.user&&previous?.user)ticket.user=previous.user;
  if(index>=0)rows[index]=ticket;else rows.unshift(ticket);
}
function ticketUserLine(ticket) {
  const user=ticket.user||{}, telegram=user.username?`@${user.username}`:'@—', panel=user.remnawave_username||'—';
  return `${telegram} · ${panel}`;
}
function ticketRow(ticket) {
  const last=ticket.messages?.[ticket.messages.length-1], adminLine=state.data.user.is_admin?`<small class="ticket-user-line">${esc(ticketUserLine(ticket))}</small>`:'';
  return `<button class="list-row ticket-row" data-nav="ticket:${ticket.id}"><span class="icon-box">${icon('support')}</span><span class="ticket-copy"><strong>${esc(ticket.subject)}</strong>${adminLine}<small>${esc(last?.text || 'Сообщений пока нет')}</small></span><span class="ticket-side"><span class="ticket-time">${formatStamp(ticket.updated_at)}</span><span class="status ${esc(ticket.status)}">${statusLabel(ticket.status)}</span></span></button>`;
}

function ticketMessage(message) {
  const adminViewer=state.data.user.is_admin, mine=adminViewer?message.is_admin:!message.is_admin;
  return `<article class="bubble ${mine ? 'mine' : 'support-message'}"><strong>${message.is_admin ? 'Поддержка' : (adminViewer ? 'Пользователь' : 'Вы')}</strong><p>${esc(message.text)}</p><time>${formatStamp(message.created_at)}</time></article>`;
}

function ticketMessages(messages) {
  return messages.length ? messages.map(ticketMessage).join('') : empty('support','Сообщений пока нет');
}

function ticketPage(id) {
  if (state.data.user.is_admin && !Array.isArray(state.cache.adminTickets)) loadSupportTickets();
  const ticket = visibleTickets().find(t => t.id === id);
  if (!ticket) return `${detailHead('Тикет', 'support')}${empty('support','Тикет не найден')}`;
  const messages = ticket.messages || [];
  const admin=state.data.user.is_admin, userInfo=admin?`<div class="ticket-user-context"><span>${ticket.user?.username?'@'+esc(ticket.user.username):'@—'}</span><span>${esc(ticket.user?.remnawave_username||'—')}</span></div>`:'';
  const statusActions=admin?`<div class="ticket-status-actions"><button class="compact-button ${ticket.status==='open'?'accent':''}" data-ticket-status="${ticket.id}:open">Открыт</button><button class="compact-button ${ticket.status==='closed'?'accent':''}" data-ticket-status="${ticket.id}:closed">Закрыт</button></div>`:'';
  const compose=ticket.status!=='closed'||admin?`<form class="chat-compose" id="chat-form"><input name="text" maxlength="5000" aria-label="Сообщение" placeholder="Сообщение…" autocomplete="off"><button class="icon-button" aria-label="Отправить">${icon('send')}</button></form>`:'<p class="notice">Тикет закрыт</p>';
  return `<section class="ticket-layout">${detailHead(ticket.subject, 'support')}<div class="ticket-context">${userInfo}<div class="ticket-state"><span class="status ${esc(ticket.status)}">${statusLabel(ticket.status)}</span>${statusActions}</div></div><div class="chat" aria-label="Переписка с поддержкой">${ticketMessages(messages)}</div>${compose}</section>`;
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
  if (key === 'payments') return `${head}<div class="list">${state.data.payments?.length ? state.data.payments.map(p => `<div class="list-row" style="cursor:default"><span class="icon-box">${['yookassa','cryptobot','lava','wata','platega','freekassa','heleket','pally'].includes(p.provider)?paymentLogo(p.provider):icon('card')}</span><span><strong>${money(p.amount_rub)}</strong><small>${esc(p.snapshot?.name || p.provider)} · ${formatDate(p.created_at)}</small></span><span class="status ${p.status}">${statusLabel(p.status)}</span></div>`).join('') : empty('card','Платежей пока нет')}</div>`;
  if (key === 'referral') { const ref = state.data.referral; return `${head}<section class="hero"><p class="eyebrow">Приглашено</p><h2 class="expiry">${ref.count}</h2><div class="hero-stats"><div class="stat-block"><small>Бонус</small><strong>${ref.referral_days} дней</strong></div><div class="stat-block"><small>Трафик</small><strong>${ref.referral_traffic_gb} ГБ</strong></div></div><div class="code">${esc(ref.link)}</div><button class="primary full-width top-gap" data-copy="${esc(ref.link)}">${icon('copy')}Скопировать ссылку</button></section>`; }
  const cached = state.cache[key];
  if (!cached) { loadMore(key); return `${head}${loadingState()}`; }
  if (key === 'servers') return `${head}<div class="list">${cached.length ? cached.map(n => `<div class="list-row" style="cursor:default"><span class="icon-box">${icon('server')}</span><span><strong>${esc(n.name || 'Node')}</strong><small>${esc(n.country_code || 'Локация не указана')}</small></span><span class="status ${n.status}">${n.status === 'online' ? 'Работает' : 'Недоступен'}</span></div>`).join('') : empty('server','Ноды не найдены')}</div>`;
  if (key === 'devices') return `${head}<div class="list">${cached.length ? cached.map((d,idx) => `<div class="list-row"><span class="icon-box">${icon('devices')}</span><span><strong>${esc(d.deviceModel || d.platform || `Устройство ${idx+1}`)}</strong><small>${esc(d.userAgent || d.hwid || 'HWID')}</small></span><button class="compact-button danger icon-only" data-delete-device="${esc(d.hwid || '')}" aria-label="Удалить устройство">${icon('trash')}</button></div>`).join('') : empty('devices','Подключённых устройств нет')}</div>`;
  return '';
}

async function loadMore(key) { try { state.cache[key] = await api(key === 'servers' ? '/api/nodes' : '/api/devices'); render(); } catch (e) { state.cache[key] = []; toast(e.message, true); render(); } }

function adminPage() {
  const groups = [
    ['Система', [
      ['globe','Язык','Язык интерфейса','language'],
      ['alert','Режим аварии','Отключить доступ пользователям','emergency'],
      ['pulse','Диагностика','Ошибки панели и платежей','diagnostics'],
      ['toggles','Управление функциями','Включение разделов','features'],
      ['gift','Триал','Срок, трафик и сквады','trial'],
	  ['clock','Доступ после окончания','Временный срок и сквады','grace'],
      ['link','Интеграции','Remnawave и платежи','integrations'],
      ['gear','Система','Реферальные бонусы','system']
    ]],
    ['Интерфейс', [
      ['edit','Редактор контента','Тексты и брендинг','content'],
	  ['devices','Sub page','Клиенты подключения','subpage'],
      ['palette','Оформление','Цветовые шаблоны','theme'],
      ['plans','Тарифы','Каталог и сквады','tariffs'],
      ['sort','Разное','Порядок пунктов','more_order']
    ]],
    ['Операции', [
      ['user','Пользователи','Поиск и управление','users'],
	  ['link','Привязка подписки','Перенос на другой Telegram ID','subscription_rebind'],
      ['megaphone','Рассылка','Создание через Telegram','broadcast'],
      ['percent','Промокоды','Скидки и лимиты','promos']
    ]]
  ];
  if (!state.cache.overview) loadAdmin('overview');
  const o = state.cache.overview;
  const menu=groups.map(([title,items])=>`<section class="admin-menu-group" aria-labelledby="admin-group-${esc(title)}"><h2 id="admin-group-${esc(title)}">${esc(title)}</h2><div class="list">${items.map(([ico,itemTitle,sub,key])=>row(ico,itemTitle,sub,`admin:${key}`)).join('')}</div></section>`).join('');
  return `${o ? `<div class="admin-metrics"><div class="metric"><span>Пользователи</span><strong>${o.users}</strong></div><div class="metric"><span>Активные</span><strong>${o.active_subscriptions}</strong></div><div class="metric"><span>Выручка</span><strong>${money((o.revenue_kopecks || 0)/100)}</strong></div><div class="metric"><span>Ошибки</span><strong>${o.diagnostics}</strong></div></div>` : loadingState()}<div class="admin-menu-groups">${menu}</div>`;
}

async function loadAdmin(key, path = `/api/admin/${key}`) { try { state.cache[key] = await api(path); render(); } catch (e) { state.cache[key] = {error:e.message}; toast(e.message,true); render(); } }
function adminBack(title, subtitle='') { return `${detailHead(title, 'admin')}${subtitle ? `<p class="detail-subtitle">${esc(subtitle)}</p>` : ''}`; }

function adminDetail(key) {
  const settingKeys = ['language','emergency','features','trial','grace','integrations','content','subpage','theme','more_order','system'];
  if (settingKeys.includes(key)) {
    if (!state.cache[`setting:${key}`]) { loadAdminSetting(key); return `${adminBack('Настройки')}${loadingState()}`; }
    return settingPage(key, state.cache[`setting:${key}`]);
  }
  if (key === 'diagnostics') return adminCollection(key,'Диагностика','/api/admin/diagnostics',diagnosticsView);
  if (key === 'tariffs') return adminCollection(key,'Тарифы','/api/admin/tariffs',adminTariffsView);
  if (key === 'users') return adminCollection(key,'Пользователи','/api/admin/users',adminUsersView);
  if (key === 'promos') return adminCollection(key,'Промокоды','/api/admin/promos',adminPromosView);
  if (key === 'broadcast') return adminCollection(key,'Рассылка','/api/admin/broadcast',broadcastView);
	if (key === 'subscription_rebind') return subscriptionRebindPage();
  return adminBack('Раздел');
}

async function ensureSquads() {
  if (state.cache.squads) return state.cache.squads;
  try { state.cache.squads = await api('/api/admin/squads'); }
  catch(e) { state.cache.squads={internal:[],external:[],error:e.message};toast(e.message,true); }
  return state.cache.squads;
}
async function loadAdminSetting(key) { try { const tasks=[api(`/api/admin/settings/${key}`)];if(key==='trial'||key==='grace')tasks.push(ensureSquads());const [setting]=await Promise.all(tasks);state.cache[`setting:${key}`]=setting;render(); } catch(e) { toast(e.message,true); } }
function adminCollection(key,title,path,view) { if (!state.cache[`admin:${key}`]) { loadAdmin(`admin:${key}`,path); return `${adminBack(title)}${loadingState()}`; } const data=state.cache[`admin:${key}`]; return adminBack(title)+view(data); }

function switchRow(name,label,checked,description='') { return `<div class="switch-row"><span><strong>${esc(label)}</strong>${description ? `<small style="display:block;color:var(--muted);margin-top:3px">${esc(description)}</small>` : ''}</span><label class="switch"><input type="checkbox" name="${esc(name)}" ${checked ? 'checked' : ''}><i></i></label></div>`; }
function field(name,label,value='',type='text',hint='',wide=true) { return `<div class="field ${wide ? 'wide' : ''}"><label for="f-${esc(name)}">${esc(label)}</label><input id="f-${esc(name)}" name="${esc(name)}" type="${type}" value="${esc(value)}">${hint ? `<p class="field-hint">${esc(hint)}</p>` : ''}</div>`; }
function textareaField(name,label,value='',hint='') { return `<div class="field wide"><label for="f-${esc(name)}">${esc(label)}</label><textarea id="f-${esc(name)}" name="${esc(name)}">${esc(value)}</textarea>${hint?`<p class="field-hint">${esc(hint)}</p>`:''}</div>`; }
function colorField(name,label,value) { return `<label class="color-field"><span>${esc(label)}</span><input type="color" name="${esc(name)}" value="${esc(value)}" aria-label="${esc(label)}"><code>${esc(value)}</code></label>`; }

function squadPicker(selectedInternal=[], selectedExternal='', prefix='squads', includeExternal=true) {
  const squads=state.cache.squads||{internal:[],external:[]}, selected=new Set(selectedInternal||[]), internal=squads.internal||[], external=squads.external||[];
  const preserved=squads.error?[...selected].map((uuid,index)=>`<input id="${prefix}-preserved-${index}" type="checkbox" name="internal_squads" value="${esc(uuid)}" data-array-field checked hidden>`).join(''):'';
  const choices=internal.length?internal.map((s,index)=>`<label class="squad-choice" for="${prefix}-squad-${index}"><input id="${prefix}-squad-${index}" type="checkbox" name="internal_squads" value="${esc(s.uuid)}" data-array-field ${selected.has(s.uuid)?'checked':''}><span>${esc(s.name)}</span></label>`).join(''):`${preserved}<p class="picker-empty">${esc(squads.error||'Внутренние сквады не найдены')}</p>`;
  const currentExternal=squads.error&&selectedExternal?`<option value="${esc(selectedExternal)}" selected>Текущий внешний сквад</option>`:'';
  const externalField=includeExternal?`<div class="field"><label for="${prefix}-external">Внешний сквад</label><select id="${prefix}-external" name="external_squad_uuid" ${squads.error?'disabled':''}><option value="">Не использовать</option>${currentExternal}${external.map(s=>`<option value="${esc(s.uuid)}" ${s.uuid===selectedExternal?'selected':''}>${esc(s.name)}</option>`).join('')}</select>${squads.error?`<input type="hidden" name="external_squad_uuid" value="${esc(selectedExternal)}">`:''}</div>`:'';
  return `<section class="squad-picker"><header><span><strong>Внутренние сквады</strong><small>Доступные серверы подписки</small></span><span class="picker-actions"><button type="button" class="text-button" data-squads-all ${squads.error?'disabled':''}>Все</button><button type="button" class="text-button" data-squads-clear ${squads.error?'disabled':''}>Снять</button></span></header><div class="squad-options">${choices}</div>${externalField}</section>`;
}

function settingsAccordion(title,subtitle,ico,content,open=false) { const visual=ico.startsWith('pay-')?paymentLogo(ico.slice(4)):icon(ico);return `<details class="settings-accordion" ${open?'open':''}><summary><span class="icon-box">${visual}</span><span><strong>${esc(title)}</strong><small>${esc(subtitle)}</small></span><span class="accordion-arrow">${icon('arrow')}</span></summary><div class="accordion-body">${content}</div></details>`; }

function subpageClientField(index,name,label,value='',type='text',hint='') { const id=`client-${index}-${name}`;return `<div class="field"><label for="${id}">${esc(label)}</label><input id="${id}" name="${name}" type="${type}" value="${esc(value)}">${hint?`<p class="field-hint">${esc(hint)}</p>`:''}</div>`; }
function subpageClientEditor(client,index) {
  const platforms=Array.isArray(client.platforms)?client.platforms:[],all=Boolean(client.all_platforms);
  return `<details class="subpage-client" data-subpage-client ${index===0?'open':''}><summary><span><strong>${esc(client.name||`Новый клиент ${index+1}`)}</strong><small>${client.enabled?'Показывается пользователям':'Выключен'}</small></span><span class="accordion-arrow">${icon('arrow')}</span></summary><div class="subpage-client-body"><div class="form-grid">${subpageClientField(index,'client_id','ID',client.id||'','text','Латиница, цифры, _ и -')}${subpageClientField(index,'client_name','Название',client.name||'')}${subpageClientField(index,'client_scheme','Схема подключения',client.scheme||'','text','Например: happ://add/')}${subpageClientField(index,'client_install_url','Ссылка установки',client.install_url||'','url','Необязательно')}</div>${switchRow('client_enabled','Клиент включён',client.enabled!==false)}${switchRow('client_featured','Рекомендуемый',client.featured,'Можно выбрать только один')}${switchRow('client_all_platforms','Для всех устройств',all)}<fieldset class="platform-picker" ${all?'disabled':''}><legend>Устройства</legend>${setupPlatformMeta.map(([id,label])=>`<label><input type="checkbox" value="${id}" data-client-platform ${platforms.includes(id)?'checked':''}><span>${esc(label)}</span></label>`).join('')}</fieldset><button type="button" class="danger-button full-width" data-remove-subpage-client="${index}">${icon('trash')}Удалить клиент</button></div></details>`;
}
function subpageForm(v) {
  const clients=Array.isArray(v.clients)?v.clients:[];
  return `<form id="subpage-form" class="admin-form"><div class="settings-group">${switchRow('include_builtins','Показывать Happ и INCY',v.include_builtins!==false,'Встроенные клиенты уже настроены для всех устройств')}</div><div class="subpage-client-list">${clients.length?clients.map(subpageClientEditor).join(''):'<p class="notice">Пользовательских клиентов пока нет. Можно оставить встроенные или добавить свои.</p>'}</div><button type="button" class="secondary full-width" data-add-subpage-client ${clients.length>=24?'disabled':''}>${icon('plus')}Добавить клиент</button><button class="primary full-width">Сохранить</button></form>`;
}
function subscriptionRebindPage() {
  return `${adminBack('Привязка подписки','Подписка переносится на новый Telegram ID и полностью исчезает у старого')}<form id="subscription-rebind-form" class="admin-form"><p class="notice">Новый пользователь должен хотя бы один раз открыть бота и отправить /start. Если у него уже есть подписка, перенос будет остановлен.</p><div class="form-grid">${field('source_telegram_id','Текущий Telegram ID','','number','Откуда забрать подписку',false)}${field('target_telegram_id','Новый Telegram ID','','number','Куда перенести подписку',false)}</div>${field('reason','Причина','','text','Сохранится в журнале действий')}<button class="primary full-width">Перепривязать подписку</button></form>`;
}

function settingPage(key, payload) {
  const v = payload.value || {}, titles = {language:'Язык',emergency:'Режим аварии',features:'Управление функциями',trial:'Триал',grace:'Доступ после окончания',integrations:'Интеграции',content:'Редактор контента',subpage:'Sub page',theme:'Оформление',more_order:'Разное',system:'Система'};
  let inner='';
  if (key === 'language') inner = `<div class="field"><label>Язык всего интерфейса</label><select name="default"><option value="ru" ${v.default==='ru'?'selected':''}>Русский</option><option value="en" ${v.default==='en'?'selected':''}>English</option></select></div>`;
  if (key === 'emergency') inner = switchRow('enabled','Аварийный режим',v.enabled,'Пользователи увидят только сообщение о работах') + textareaField('message','Сообщение пользователям',v.message||state.data.content.emergency_message||'Сервис временно недоступен.');
  if (key === 'features') inner = Object.entries({trial:'Бесплатный период',server_status:'Статус серверов',devices:'Устройства',payments:'Платежи',referrals:'Реферальная система',promo_codes:'Промокоды',support:'Поддержка'}).map(([n,l])=>switchRow(n,l,v[n])).join('');
  if (key === 'trial') inner = `<div class="form-grid">${switchRow('enabled','Триал включён',v.enabled)}${field('days','Количество дней',v.days,'number','',false)}${field('traffic_gb','Трафик, ГБ (0 = безлимит)',v.traffic_gb,'number','',false)}${field('device_limit','Устройств',v.device_limit,'number','',false)}</div>${squadPicker(v.internal_squads,v.external_squad_uuid,'trial')}`;
	if (key === 'grace') inner = `<p class="notice">После окончания основной подписки выбранные сквады выдаются один раз на указанный срок.</p><div class="form-grid">${switchRow('enabled','Временный доступ включён',v.enabled)}${field('days','Срок, дней',v.days||2,'number','От 1 до 365',false)}</div>${squadPicker(v.internal_squads, '', 'grace', false)}`;
  if (key === 'content') return `${adminBack(titles[key])}${contentForms(v)}`;
	if (key === 'subpage') return `${adminBack(titles[key],'Своя страница подключения и клиенты для разных устройств')}${subpageForm(v)}`;
  if (key === 'system') inner = `<section class="settings-group"><header><strong>Реферальная система</strong><small>Бонус владельцу ссылки после приглашения</small></header><div class="form-grid">${field('referral_days','Бонус дней',v.referral_days,'number','',false)}${field('referral_traffic_gb','Бонус трафика, ГБ',v.referral_traffic_gb,'number','',false)}${switchRow('reward_after_payment','Только после первой оплаты',v.reward_after_payment,'Без оплаты приглашённого бонус не начисляется')}</div></section>`;
  if (key === 'more_order') { const items=v.items||[]; inner=`<p class="notice">Изменяйте порядок кнопками. Видимость разделов задаётся в управлении функциями.</p><div class="list" id="order-list">${items.map((name,idx)=>`<div class="list-row" data-order-item="${name}"><span class="icon-box">${icon(moreItems[name]?.[0]||'info')}</span><span><strong>${esc(moreItems[name]?.[1]||name)}</strong></span><span class="order-actions"><button type="button" class="compact-button icon-only" data-move="${idx}:up" aria-label="Выше">${icon('arrow-up')}</button><button type="button" class="compact-button icon-only" data-move="${idx}:down" aria-label="Ниже">${icon('arrow-down')}</button></span></div>`).join('')}</div><input type="hidden" name="items" value="${esc(items.join(','))}">`; }
  if (key === 'theme') { const templates=payload.theme_templates||{},names={telegram:'Telegram',graphite:'Графит',emerald:'Изумруд',sand:'Песок',ocean:'Океан',violet:'Фиолетовый',ruby:'Рубин',steel:'Сталь'}; inner=`<div class="theme-grid">${Object.entries(templates).map(([name,t])=>`<button type="button" class="theme-option ${v.template===name?'active':''}" data-theme="${name}" data-theme-values="${esc(JSON.stringify(t))}"><span class="swatches"><i style="background:${t.background}"></i><i style="background:${t.surface}"></i><i style="background:${t.accent}"></i></span><strong>${esc(names[name]||name)}</strong></button>`).join('')}</div><input type="hidden" name="template" value="${esc(v.template||'telegram')}"><div class="color-grid">${colorField('accent','Акцент',v.accent)}${colorField('background','Фон',v.background)}${colorField('surface','Карточки',v.surface)}${colorField('surface_alt','Поля',v.surface_alt)}${colorField('text','Текст',v.text)}${colorField('muted','Вторичный текст',v.muted)}</div>`; }
  if (key === 'integrations') return `${adminBack(titles[key])}${integrationForms(v)}`;
  return `${adminBack(titles[key])}<form class="admin-form" data-setting-form="${key}">${inner}<button class="primary" style="width:100%;margin-top:16px">Сохранить</button></form>`;
}

function integrationForms(v) {
  const groups=[
    ['remnawave','Remnawave','Панель подписок',[['url','URL панели','url'],['token','API Token','password'],['webhook_secret','Секрет webhook','password']]],
    ['yookassa','ЮKassa','Банковские карты и СБП',[['shop_id','Shop ID','text'],['secret_key','Secret Key','password'],['email','Email для чеков','email']]],
    ['cryptobot','CryptoBot','Криптовалюта через Telegram',[['token','API Token','password']]],
    ['lava','LAVA','Платёжная форма LAVA Business',[['shop_id','Shop ID','text'],['secret_key','Секретный ключ','password'],['additional_key','Дополнительный ключ','password']]],
    ['wata','WATA','Карты и СБП через WATA',[['access_token','Access token','password'],['api_url','API URL','url']]],
    ['platega','Platega','Платёжная форма Platega',[['merchant_id','Merchant ID','text'],['secret_key','API ключ','password'],['api_url','API URL','url']]],
    ['freekassa','FreeKassa','Платёжная форма FreeKassa',[['shop_id','ID магазина','text'],['secret_word','Секретное слово','password'],['secret_word2','Секретное слово 2','password']]],
    ['heleket','Heleket','Криптовалютная платёжная форма',[['merchant_id','Merchant UUID','text'],['api_key','Payment API key','password'],['api_url','API URL','url']]],
    ['pally','Pally','Карты и СБП через Pally',[['shop_id','Shop ID','text'],['api_token','API token','password'],['api_url','API URL','url']]],
    ['notifications','Бот уведомлений','Отдельный чат администратора',[['bot_token','Токен бота','password'],['chat_id','Chat ID','text']]]
  ];
  const icons={remnawave:'link',yookassa:'pay-yookassa',cryptobot:'pay-cryptobot',lava:'pay-lava',wata:'pay-wata',platega:'pay-platega',freekassa:'pay-freekassa',heleket:'pay-heleket',pally:'pay-pally',notifications:'send'};
  return `<div class="accordion-stack">${groups.map(([key,title,description,fields],index)=>{const c=v[key]||{};const webhookHint=key==='remnawave'?'<p class="field-hint">Webhook: /api/webhooks/remnawave · заголовок X-Webhook-Secret</p>':['lava','wata','platega','freekassa','heleket','pally'].includes(key)?`<p class="field-hint">Webhook: /api/webhooks/payments/${key}</p>`:'';const body=`<form data-setting-form="integrations">${switchRow(`${key}.enabled`,'Включено',c.enabled)}${fields.map(([n,l,t])=>field(`${key}.${n}`,l,c[n]||'',t)).join('')}${webhookHint}${key==='cryptobot'?switchRow('cryptobot.testnet','Тестовая сеть',c.testnet):''}<div class="form-actions">${key!=='notifications'?`<button type="button" class="secondary" data-test-integration="${key}">Проверить</button>`:''}<button class="primary">Сохранить</button></div></form>`;return settingsAccordion(title,c.enabled?'Подключено':description,icons[key],body,index===0)}).join('')}</div>`;
}

function contentForms(v) {
  const legacyStart=`<b>${v.start_title||'Добро пожаловать в TGS VPN'}</b>\n\n${v.start_text||'Управляйте подпиской в Mini App.'}`;
  const home=`<form data-setting-form="content"><div class="form-grid">${field('no_subscription_title','Нет подписки — заголовок',v.no_subscription_title||'')}${field('subscription_expired_title','Подписка закончилась — заголовок',v.subscription_expired_title||'')}${field('buy_subscription_button','Купить подписку — кнопка',v.buy_subscription_button||'')}${field('renew_button','Продлить — кнопка',v.renew_button||'')}${field('connect_button','Подключиться — кнопка',v.connect_button||'')}${field('traffic_used_label','Подпись трафика',v.traffic_used_label||'')}</div><button class="primary full-width">Сохранить</button></form>`;
  const support=`<form data-setting-form="content">${textareaField('support_welcome','Текст над созданием обращения',v.support_welcome||'')}<div class="form-grid">${field('support_create_button','Кнопка нового тикета',v.support_create_button||'')}${field('support_tickets_title','Заголовок пользователя',v.support_tickets_title||'')}${field('support_admin_title','Заголовок администратора',v.support_admin_title||'')}${field('support_admin_hint','Подсказка администратору',v.support_admin_hint||'')}${field('support_empty','Пустой список',v.support_empty||'')}</div><button class="primary full-width">Сохранить</button></form>`;
  const connect=`<form data-setting-form="content"><div class="form-grid">${field('connect_install_title','Шаг 1 — заголовок',v.connect_install_title||'')}${field('connect_install_hint','Шаг 1 — описание',v.connect_install_hint||'')}${field('connect_add_title','Шаг 2 — заголовок',v.connect_add_title||'')}${field('connect_add_hint','Шаг 2 — описание',v.connect_add_hint||'')}${field('connect_use_title','Шаг 3 — заголовок',v.connect_use_title||'')}${field('connect_use_hint','Шаг 3 — описание',v.connect_use_hint||'')}${field('connect_recommended','Метка рекомендации',v.connect_recommended||'')}${field('connect_open_button','Добавить подписку — кнопка',v.connect_open_button||'')}${field('connect_install_button','Скачать — кнопка',v.connect_install_button||'')}${field('connect_copy_button','Копировать — кнопка',v.connect_copy_button||'')}${field('connect_empty','Нет клиентов',v.connect_empty||'')}</div><button class="primary full-width">Сохранить</button></form>`;
  const start=`<form data-setting-form="content">${telegramMessageField('start_message','Сообщение /start',v.start_message||legacyStart)}${telegramButtonEditor('trial_button','Бесплатный период',v)}${telegramButtonEditor('cabinet_button','Личный кабинет',v)}${telegramButtonEditor('support_button','Поддержка',v)}<button class="primary full-width">Сохранить</button></form>`;
  const notifications=`<form data-setting-form="content">${telegramMessageField('payment_success_message','Успешная оплата',v.payment_success_message||'')}${telegramMessageField('grace_access_message','Доступ после окончания',v.grace_access_message||'','Переменная: {days}')}${telegramMessageField('support_reply_message','Ответ поддержки',v.support_reply_message||'','Переменная: {subject}')}${telegramMessageField('full_block_message','Полная блокировка',v.full_block_message||'')}${telegramMessageField('subscription_rebound_old_message','Перенос — старому владельцу',v.subscription_rebound_old_message||'')}${telegramMessageField('subscription_rebound_new_message','Перенос — новому владельцу',v.subscription_rebound_new_message||'')}${telegramMessageField('myid_message','Команда /myid',v.myid_message||'','Переменная: {id}')}${telegramMessageField('emergency_message','Аварийный режим',v.emergency_message||'')}<button class="primary full-width">Сохранить</button></form>`;
  const adminAlerts=`<form data-setting-form="content">${telegramMessageField('trial_admin_message','Выдан пробный период',v.trial_admin_message||'','Переменные: {name}, {id}')}${telegramMessageField('new_ticket_admin_message','Новый тикет',v.new_ticket_admin_message||'','Переменные: {subject}, {name}, {id}')}${telegramMessageField('ticket_message_admin_message','Новое сообщение в тикете',v.ticket_message_admin_message||'','Переменная: {subject}')}${telegramMessageField('payment_admin_message','Новая оплата',v.payment_admin_message||'','Переменные: {name}, {amount}, {provider}')}<button class="primary full-width">Сохранить</button></form>`;
  const broadcast=`<form data-setting-form="content">${telegramMessageField('admin_only_message','Нет прав администратора',v.admin_only_message||'')}${telegramMessageField('broadcast_prompt_message','Запрос сообщения',v.broadcast_prompt_message||'')}${telegramMessageField('broadcast_draft_error_message','Ошибка черновика',v.broadcast_draft_error_message||'')}${telegramMessageField('broadcast_save_error_message','Ошибка сохранения',v.broadcast_save_error_message||'')}${telegramMessageField('broadcast_preview_error_message','Ошибка предпросмотра',v.broadcast_preview_error_message||'')}${telegramMessageField('broadcast_edit_message','Повторный ввод',v.broadcast_edit_message||'')}${telegramMessageField('broadcast_confirmed_message','Сообщение подтверждено',v.broadcast_confirmed_message||'')}${telegramMessageField('broadcast_start_error_message','Ошибка запуска',v.broadcast_start_error_message||'')}${telegramMessageField('broadcast_complete_message','Рассылка завершена',v.broadcast_complete_message||'','Переменные: {sent}, {failed}')}<button class="primary full-width">Сохранить</button></form>`;
  const brand=`<form data-setting-form="content">${field('brand','Название',v.brand||'')}${field('logo_url','URL логотипа',v.logo_url||'','url')}<button class="primary full-width">Сохранить</button></form>`;
  const groups=[['Главный экран','Статусы и кнопки подписки','home',home],['Страница подключения','Тексты выбора клиентов','link',connect],['Поддержка','Тексты списка обращений','support',support],['Чат и /start','HTML, премиум-эмодзи и кнопки','send',start],['Уведомления','Все сообщения пользователям','info',notifications],['Администратору','Триал, тикеты и платежи','admin',adminAlerts],['Рассылка','Системные сообщения в чате','megaphone',broadcast],['Брендинг','Название и логотип','palette',brand]];
  return `<div class="accordion-stack">${groups.map(([title,sub,ico,body],index)=>settingsAccordion(title,sub,ico,body,index===0)).join('')}</div>`;
}

function telegramMessageField(name,label,value='',variables='') {
  const help=`Telegram HTML: &lt;b&gt;, &lt;i&gt;, &lt;u&gt;, &lt;s&gt;, &lt;code&gt;, ссылки и &lt;tg-emoji emoji-id=&quot;ID&quot;&gt;🙂&lt;/tg-emoji&gt;${variables?` · ${esc(variables)}`:''}`;
  return `<div class="field wide telegram-message-field"><label for="f-${esc(name)}">${esc(label)}</label><textarea class="telegram-source" id="f-${esc(name)}" name="${esc(name)}" spellcheck="false">${esc(value)}</textarea><p class="field-hint">${help}</p></div>`;
}

function telegramButtonEditor(prefix,label,v) {
  const style=v[`${prefix}_style`]||'', emoji=v[`${prefix}_emoji_id`]||'';
  return `<section class="telegram-button-editor"><strong>${esc(label)}</strong><div class="form-grid">${field(prefix,'Название',v[prefix]||'') }<div class="field"><label for="f-${prefix}-style">Цвет</label><select id="f-${prefix}-style" name="${prefix}_style"><option value="" ${!style?'selected':''}>Обычная</option><option value="primary" ${style==='primary'?'selected':''}>Синяя</option><option value="success" ${style==='success'?'selected':''}>Зелёная</option><option value="danger" ${style==='danger'?'selected':''}>Красная</option></select></div>${field(`${prefix}_emoji_id`,'ID премиум-эмодзи',emoji,'text','Необязательно')}</div></section>`;
}

function diagnosticsView(rows) { if (!Array.isArray(rows)) return empty('alert',rows.error||'Ошибка'); return `<div class="list">${rows.length ? rows.map(d=>`<div class="list-row"><span class="icon-box">${icon('pulse')}</span><span><strong>${esc(d.source)}</strong><small>${esc(d.message)} · ${formatDate(d.created_at)}</small></span>${d.resolved?'<span class="status active">Решено</span>':`<button class="compact-button" data-resolve="${d.id}">Решено</button>`}</div>`).join('') : empty('pulse','Ошибок нет')}</div>`; }
function adminTariffsView(rows) { if (!Array.isArray(rows)) return empty('plans','Ошибка'); return `<button class="primary" style="width:100%;margin-bottom:12px" data-admin-tariff="new">${icon('plus')}Создать тариф</button><div class="list">${rows.map(t=>`<button class="list-row" data-admin-tariff="${t.id}"><span class="icon-box">${icon('plans')}</span><span><strong>${esc(t.name)}</strong><small>${t.days} дней · ${money(t.price_rub)}${t.active?'':' · выключен'}</small></span>${icon('arrow')}</button>`).join('')}</div>`; }
function adminUsersView(rows) { if (!Array.isArray(rows)) return empty('user','Ошибка'); return `<form id="user-search" class="field" style="margin-top:0"><label>Поиск</label><input name="q" placeholder="Имя, username или Telegram ID"></form><div class="list">${rows.length?rows.map(u=>`<button class="list-row" data-admin-user="${u.id}">${avatar(u,'avatar')}<span><strong>${esc(u.first_name)}</strong><small>${u.username?'@'+esc(u.username):u.telegram_id} · ${statusLabel(u.subscription.status)}</small></span>${icon('arrow')}</button>`).join(''):empty('user','Пользователи не найдены')}</div>`; }
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
  state.cache.checkoutPromo={sequence:(state.cache.checkoutPromo?.sequence||0)+1,timer:null};
  const promo=state.data.features.promo_codes?`<div class="field wide checkout-promo"><label for="checkout-promo">Промокод</label><div class="promo-input-wrap"><input id="checkout-promo" name="promo_code" type="text" autocomplete="off" autocapitalize="characters" spellcheck="false"><span class="promo-discount" aria-hidden="true"></span></div><p class="promo-feedback" aria-live="polite">Необязательно</p></div>`:'';
  openModal(`Оплата — ${tariff.name}`, `<form id="checkout-form" data-base-price="${tariff.price_rub}" data-methods-ready="${methods.length?'1':'0'}"><input type="hidden" name="tariff_id" value="${tariff.id}"><div class="choice-list">${methods.length?methods.map((m,i)=>`<label class="choice payment-choice"><input type="radio" name="provider" value="${m.id}" ${i===0?'checked':''}>${paymentLogo(m.id)}<span><strong>${esc(m.name)}</strong><small>${esc(m.description||'Онлайн-оплата')}</small></span></label>`).join(''):empty('card','Способы оплаты ещё не подключены')}</div>${promo}<button class="primary checkout-submit" ${methods.length?'':'disabled'}><span>Оплатить</span><strong class="checkout-price">${money(tariff.price_rub)}</strong></button></form>`);
}
function setCheckoutPrice(form,value,animate=false){const price=$('.checkout-price',form);if(!price)return;const next=money(value);if(price.textContent===next)return;price.textContent=next;if(animate&&!matchMedia('(prefers-reduced-motion: reduce)').matches){price.classList.remove('price-changing');void price.offsetWidth;price.classList.add('price-changing')}}
function setPromoState(form,stateName,message='',discount=0,price=null){
  const feedback=$('.promo-feedback',form),badge=$('.promo-discount',form),button=$('.checkout-submit',form),input=$('[name="promo_code"]',form),available=form.dataset.methodsReady==='1';
  if(!feedback||!badge||!button)return;
  feedback.className=`promo-feedback ${stateName}`;feedback.textContent=message||'Необязательно';
  badge.textContent=discount?`-${discount}%`:'';badge.classList.toggle('visible',Boolean(discount));
  input?.setAttribute('aria-invalid',stateName==='error'?'true':'false');
  button.disabled=!available||['checking','error'].includes(stateName);
  setCheckoutPrice(form,price??Number(form.dataset.basePrice),stateName==='success');
}
function validateCheckoutPromo(input){
  const form=input.closest('#checkout-form'),promoState=state.cache.checkoutPromo||(state.cache.checkoutPromo={sequence:0,timer:null}),code=input.value.trim();
  clearTimeout(promoState.timer);promoState.sequence+=1;const sequence=promoState.sequence;
  if(!code){setPromoState(form,'idle','Необязательно');return;}
  setPromoState(form,'checking','Проверяем промокод…');
  promoState.timer=setTimeout(async()=>{try{const out=await api('/api/promos/validate',{method:'POST',body:JSON.stringify({tariff_id:form.elements.tariff_id.value,promo_code:code})});if(sequence!==promoState.sequence||!document.contains(form))return;input.value=out.code;setPromoState(form,'success','Промокод применен',Number(out.discount_percent),Number(out.price_rub));tg?.HapticFeedback?.notificationOccurred?.('success')}catch(error){if(sequence!==promoState.sequence||!document.contains(form))return;setPromoState(form,'error',error.message||'Промокод не подходит') }},360);
}
function ticketModal() { openModal('Новый тикет', `<form id="ticket-form">${field('subject','Тема','','text','Коротко опишите вопрос')}<div class="field"><label for="ticket-description">Описание</label><textarea id="ticket-description" name="description" maxlength="5000" required placeholder="Что случилось?"></textarea></div><button class="primary" style="width:100%">Отправить</button></form>`); }

async function tariffAdminModal(id) { openModal(id==='new'?'Новый тариф':'Редактирование тарифа',loadingState('Загружаем сквады'));try{await ensureSquads();const existing=id==='new'?{name:'',description:'',price_rub:199,days:30,traffic_gb:100,device_limit:1,internal_squads:[],external_squad_uuid:'',active:true,pinned:false,position:0}:state.cache['admin:tariffs'].find(t=>t.id===id);openModal(id==='new'?'Новый тариф':'Редактирование тарифа',`<form id="tariff-admin-form" data-id="${id}"><div class="form-grid">${field('name','Название',existing.name)}${field('description','Описание',existing.description)}${field('price_rub','Цена, ₽',existing.price_rub,'number','',false)}${field('days','Дней',existing.days,'number','',false)}${field('traffic_gb','Трафик, ГБ (0 = безлимит)',existing.traffic_gb,'number','',false)}${field('device_limit','Устройств',existing.device_limit,'number','',false)}</div>${squadPicker(existing.internal_squads,existing.external_squad_uuid,`tariff-${id}`)}${switchRow('active','Тариф включён',existing.active)}${switchRow('pinned','Закрепить первым',existing.pinned)}${field('position','Позиция',existing.position,'number')}<div class="modal-actions">${id!=='new'?`<button type="button" class="danger-button" data-remove-tariff="${id}">Удалить</button>`:'<button type="button" class="secondary" data-close-modal>Отмена</button>'}<button class="primary">Сохранить</button></div></form>`);}catch(e){closeModal(true);toast(e.message,true)} }
function userAdminModal(id){const u=state.cache['admin:users'].find(x=>String(x.id)===String(id));openModal(u.first_name,`<form id="user-admin-form" data-id="${u.id}"><div class="code">Telegram ID: ${u.telegram_id}<br>${u.username?'@'+esc(u.username):'Без username'}<br>Подписка до: ${formatDate(u.subscription.expires_at)}<br>Рефералов: ${u.referral_count||0} · код ${esc(u.referral_code||'—')}</div><div class="field"><label>Статус VPN в Remnawave</label><select name="subscription_status"><option value="">Не менять</option><option value="ACTIVE">Включить VPN</option><option value="DISABLED">Отключить VPN</option></select></div><div class="form-grid">${field('add_days','Добавить дней',0,'number','',false)}${field('add_traffic_gb','Добавить ГБ',0,'number','',false)}${field('device_limit','Лимит устройств',u.subscription.device_limit,'number','',false)}</div>${switchRow('blocked','Заблокировать кабинет',u.is_blocked,'Не отключает VPN')}${switchRow('admin','Администратор',u.is_admin)}${field('reason','Причина изменения','','text','Сохранится в журнале действий')}<div class="modal-actions"><button type="button" class="danger-button" data-full-block-user>Полная блокировка</button><button class="primary">Сохранить</button></div></form>`)}
function promoModal(){openModal('Новый промокод',`<form id="promo-form">${field('code','Код','','text')}${field('discount_percent','Скидка, %',10,'number')}${field('max_uses','Лимит использований (0 = без лимита)',0,'number')}${switchRow('active','Активен',true)}<button class="primary" style="width:100%;margin-top:14px">Создать</button></form>`)}

function collectBroadcastButtons(form) { return $$('.broadcast-button',form).map(row=>({text:$('[name="button_text"]',row).value.trim(),url:$('[name="button_url"]',row).value.trim(),style:$('[name="button_style"]',row).value,icon_custom_emoji_id:$('[name="button_emoji"]',row).value.trim()})); }
async function saveBroadcastButtons(form) { const id=form.dataset.id,buttons=collectBroadcastButtons(form);const draft=await api(`/api/admin/broadcast/${id}/buttons`,{method:'PUT',body:JSON.stringify({buttons})});state.cache['admin:broadcast'].draft=draft;return draft; }

document.addEventListener('click', async event => {
  const platformTrigger=event.target.closest('[data-setup-platform-trigger]');if(platformTrigger){setSetupPlatformMenu(platformTrigger.getAttribute('aria-expanded')!=='true');return;}
  const platformOption=event.target.closest('[data-setup-platform-option]');if(platformOption){state.cache.setupPlatform=platformOption.dataset.setupPlatformOption;state.cache.setupClient='';setSetupPlatformMenu(false);render();requestAnimationFrame(()=>$('[data-setup-platform-trigger]')?.focus());tg?.HapticFeedback?.selectionChanged?.();return;}
  if($('.setup-platform-menu:not([hidden])')&&!event.target.closest('[data-platform-control]'))setSetupPlatformMenu(false);
  const navButton=event.target.closest('[data-nav]'); if(navButton){navigate(navButton.dataset.nav);return;}
  if(event.target.closest('[data-close-modal]') || (event.target.matches('[data-modal-backdrop]'))){closeModal();return;}
  const tariffChoice=event.target.closest('[data-select-tariff]');if(tariffChoice){const id=tariffChoice.dataset.selectTariff;if(id===state.selectedTariffID)return;state.selectedTariffID=id;$$('[data-select-tariff]').forEach(card=>{const selected=card.dataset.selectTariff===id;card.classList.toggle('selected',selected);card.setAttribute('aria-pressed',String(selected));const marker=$('.tariff-selection',card);if(marker)marker.innerHTML=selected?`${icon('check')}Выбрано`:''});const tariff=state.data.tariffs.find(item=>item.id===id);const pay=$('.tariff-pay');if(tariff&&pay){pay.dataset.buy=id;$('span',pay).textContent=`${tr('buy')} · ${money(tariff.price_rub)}`;}tg?.HapticFeedback?.selectionChanged?.();return;}
  const buy=event.target.closest('[data-buy]');if(buy){buyModal(buy.dataset.buy);return;}
	const setupClient=event.target.closest('[data-setup-client]');if(setupClient){state.cache.setupClient=setupClient.dataset.setupClient;render();return;}
	const connectButton=event.target.closest('[data-open-connect-handoff],[data-retry-connect]');if(connectButton){const platformID=state.cache.setupPlatform,client=setupClients(platformID).find(item=>item.id===state.cache.setupClient),url=state.data.user.subscription.subscription_url||'',shouldOpen=connectButton.matches('[data-open-connect-handoff]');if(!client||!url){toast('Ссылка подключения недоступна',true);return;}const old=connectButton.innerHTML;connectButton.disabled=true;if(shouldOpen)connectButton.innerHTML=`${icon('loader')}<span>Открываем браузер…</span>`;const handoff=await loadConnectHandoff(platformID,client,url,!shouldOpen);if(handoff?.url&&shouldOpen)openBrowserLink(handoff.url);else if(handoff?.error)toast(handoff.error,true);if(document.contains(connectButton)){connectButton.disabled=false;connectButton.innerHTML=old}return;}
  const action=event.target.closest('[data-action]')?.dataset.action;
  if(action==='new-ticket'){ticketModal();return;} if(action==='new-promo'){promoModal();return;}
  if(action==='trial'){const button=event.target.closest('[data-action="trial"]');const old=button?.innerHTML;if(button){button.disabled=true;button.innerHTML=`${icon('loader')}<span>Подключаем…</span>`}try{liveMuteBootstrapUntil=Date.now()+900;await api('/api/trial',{method:'POST',body:'{}'});await loadBootstrap();toast('Пробный период активирован');}catch(e){if(button){button.disabled=false;button.innerHTML=old}toast(e.message,true)}return;}
  if(action==='connect'){navigate('connect');return;}
  const copy=event.target.closest('[data-copy]');if(copy){await navigator.clipboard.writeText(copy.dataset.copy);toast('Ссылка скопирована');return;}
  const device=event.target.closest('[data-delete-device]');if(device&&confirm('Удалить это устройство?')){try{await api(`/api/devices/${encodeURIComponent(device.dataset.deleteDevice)}`,{method:'DELETE'});state.cache.devices=null;loadMore('devices');toast('Устройство удалено');}catch(e){toast(e.message,true)}return;}
  const allSquads=event.target.closest('[data-squads-all]');if(allSquads){$$('[data-array-field]',allSquads.closest('.squad-picker')).forEach(input=>input.checked=true);return;}
  const clearSquads=event.target.closest('[data-squads-clear]');if(clearSquads){$$('[data-array-field]',clearSquads.closest('.squad-picker')).forEach(input=>input.checked=false);return;}
	if(event.target.closest('[data-add-subpage-client]')){const setting=state.cache['setting:subpage'];if(setting){const clients=setting.value.clients||(setting.value.clients=[]);if(clients.length<24){clients.push({id:`client-${clients.length+1}`,name:'',scheme:'',install_url:'',enabled:true,featured:false,all_platforms:false,platforms:[]});render()}}return;}
	const removeClient=event.target.closest('[data-remove-subpage-client]');if(removeClient){const setting=state.cache['setting:subpage'];if(setting&&confirm('Удалить этот клиент?')){setting.value.clients.splice(Number(removeClient.dataset.removeSubpageClient),1);render()}return;}
  const tariff=event.target.closest('[data-admin-tariff]');if(tariff){await tariffAdminModal(tariff.dataset.adminTariff);return;}
  const user=event.target.closest('[data-admin-user]');if(user){userAdminModal(user.dataset.adminUser);return;}
  const fullBlock=event.target.closest('[data-full-block-user]');if(fullBlock){if(!confirm('Заблокировать кабинет и отключить VPN?'))return;const form=fullBlock.closest('form');form.elements.blocked.checked=true;form.elements.subscription_status.value='DISABLED';form.requestSubmit();return;}
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
    if(form.id==='checkout-form'){const button=$('.checkout-submit',form),old=button.innerHTML;button.disabled=true;button.innerHTML=`${icon('loader')}<span>Создаём счёт…</span>`;try{const body=Object.fromEntries(new FormData(form));const out=await api('/api/payments/checkout',{method:'POST',body:JSON.stringify(body)});closeModal();openBrowserLink(out.pay_url);toast(`Счёт на ${money(out.amount_rub)} создан`)}finally{if(document.contains(button)){button.disabled=false;button.innerHTML=old}}return;}
    if(form.id==='ticket-form'){const body=Object.fromEntries(new FormData(form));liveMuteSupportUntil=Date.now()+800;const ticket=await api('/api/tickets',{method:'POST',body:JSON.stringify(body)});state.data.tickets.unshift(ticket);closeModal();navigate(`ticket:${ticket.id}`);toast('Тикет создан');return;}
    if(form.id==='chat-form'){const input=form.elements.text,draft=input.value.trim();if(!draft)return;const id=state.page.split(':')[1],currentTicket=visibleTickets().find(t=>t.id===id),oldCount=Math.max(0,currentTicket?.messages?.length||0),button=$('button',form),buttonHTML=button.innerHTML;input.disabled=true;button.disabled=true;button.innerHTML=icon('loader');try{liveMuteSupportUntil=Date.now()+800;const ticket=await api(`/api/tickets/${id}/messages`,{method:'POST',body:JSON.stringify({text:draft})});replaceTicket(ticket);const chat=$('.chat'),messages=ticket.messages||[];if(chat){if(messages.length>=oldCount&&oldCount>0){messages.slice(oldCount).forEach(message=>chat.insertAdjacentHTML('beforeend',ticketMessage(message)))}else{chat.innerHTML=ticketMessages(messages)}chat.scrollTo({top:chat.scrollHeight,behavior:matchMedia('(prefers-reduced-motion: reduce)').matches?'auto':'smooth'})}const status=$('.ticket-state > .status');if(status){status.className=`status ${ticket.status}`;status.textContent=statusLabel(ticket.status)}input.value='';}finally{if(document.contains(form)){input.disabled=false;button.disabled=false;button.innerHTML=buttonHTML;input.focus()}}return;}
	if(form.id==='subpage-form'){const body=collectSubpageForm(form),previous=state.cache['setting:subpage'];liveMuteBootstrapUntil=Date.now()+900;const out=await api('/api/admin/settings/subpage',{method:'PUT',body:JSON.stringify({value:body})});state.cache['setting:subpage']={...previous,...out};state.data.subpage=out.value;toast('Клиенты сохранены');return;}
	if(form.id==='subscription-rebind-form'){const body=collectForm(form);body.source_telegram_id=Number(body.source_telegram_id);body.target_telegram_id=Number(body.target_telegram_id);if(!confirm(`Перенести подписку с ${body.source_telegram_id} на ${body.target_telegram_id}?`))return;const button=$('button[type="submit"]',form),old=button.innerHTML;button.disabled=true;button.innerHTML=`${icon('loader')}Переносим…`;try{await api('/api/admin/subscriptions/rebind',{method:'POST',body:JSON.stringify(body)});form.reset();toast('Подписка перепривязана')}finally{button.disabled=false;button.innerHTML=old}return;}
    if(form.dataset.settingForm){const key=form.dataset.settingForm,body=collectForm(form);const cacheKey=`setting:${key}`,previous=state.cache[cacheKey]||{};liveMuteBootstrapUntil=Date.now()+900;const out=await api(`/api/admin/settings/${key}`,{method:'PUT',body:JSON.stringify({value:body})});state.cache[cacheKey]=key==='theme'?{...previous,...out,theme_templates:out.theme_templates||previous.theme_templates}:out;if(key==='theme'){state.data.theme={...state.data.theme,...out.value};applyTheme(state.data.theme)}if(key==='language')state.data.language=out.value;if(key==='emergency')state.data.emergency=out.value;if(key==='features')state.data.features={...state.data.features,...out.value};if(key==='grace')state.data.grace=out.value;if(key==='content')state.data.content={...state.data.content,...out.value};if(key==='more_order')state.data.more_order=out.value.items;toast('Сохранено');if(!['integrations','content'].includes(key))render();return;}
    if(form.id==='tariff-admin-form'){const id=form.dataset.id,body=collectForm(form);body.price_rub=Number(body.price_rub);['days','traffic_gb','device_limit','position'].forEach(k=>body[k]=Number(body[k]));const method=id==='new'?'POST':'PUT',path=id==='new'?'/api/admin/tariffs':`/api/admin/tariffs/${id}`;liveMuteBootstrapUntil=Date.now()+900;await api(path,{method,body:JSON.stringify(body)});closeModal();state.cache['admin:tariffs']=null;await loadAdmin('admin:tariffs','/api/admin/tariffs');toast('Тариф сохранён');return;}
    if(form.id==='user-admin-form'){const body=collectForm(form);['add_days','add_traffic_gb','device_limit'].forEach(k=>body[k]=Number(body[k]));await api(`/api/admin/users/${form.dataset.id}`,{method:'PATCH',body:JSON.stringify(body)});closeModal();state.cache['admin:users']=null;await loadAdmin('admin:users','/api/admin/users');toast('Пользователь обновлён');return;}
    if(form.id==='promo-form'){const body=collectForm(form);body.discount_percent=Number(body.discount_percent);body.max_uses=Number(body.max_uses);await api('/api/admin/promos',{method:'POST',body:JSON.stringify(body)});closeModal();state.cache['admin:promos']=null;await loadAdmin('admin:promos','/api/admin/promos');toast('Промокод создан');return;}
    if(form.id==='user-search'){state.cache['admin:users']=null;await loadAdmin('admin:users',`/api/admin/users?q=${encodeURIComponent(form.elements.q.value)}`);return;}
  } catch(e) { toast(e.message,true); }
});

function collectForm(form) { const out={};$$('[name]',form).forEach(input=>{const path=input.name.split('.');let cursor=out;path.slice(0,-1).forEach(part=>cursor=cursor[part]||(cursor[part]={}));const key=path[path.length-1];if(input.hasAttribute('data-array-field')){if(!Array.isArray(cursor[key]))cursor[key]=[];if(input.checked)cursor[key].push(input.value);return;}let value=input.type==='checkbox'?input.checked:input.value;if(input.type==='number')value=Number(value);cursor[key]=value;});if(form.dataset.settingForm==='more_order')out.items=(out.items||'').split(',').filter(Boolean);return out;}

function collectSubpageForm(form){return {include_builtins:Boolean(form.elements.include_builtins?.checked),clients:$$('[data-subpage-client]',form).map(row=>({id:$('[name="client_id"]',row).value.trim().toLowerCase(),name:$('[name="client_name"]',row).value.trim(),scheme:$('[name="client_scheme"]',row).value.trim(),install_url:$('[name="client_install_url"]',row).value.trim(),enabled:Boolean($('[name="client_enabled"]',row).checked),featured:Boolean($('[name="client_featured"]',row).checked),all_platforms:Boolean($('[name="client_all_platforms"]',row).checked),platforms:$$('[data-client-platform]:checked',row).map(input=>input.value)}))};}

document.addEventListener('click',async event=>{const status=event.target.closest('[data-ticket-status]');if(!status)return;const [id,value]=status.dataset.ticketStatus.split(':');try{liveMuteSupportUntil=Date.now()+800;const ticket=await api(`/api/admin/tickets/${id}`,{method:'PATCH',body:JSON.stringify({status:value})});replaceTicket(ticket);toast('Статус изменён');render()}catch(e){toast(e.message,true)}});
document.addEventListener('keydown', event => { const menu=$('.setup-platform-menu:not([hidden])');if(menu&&event.key==='Escape'){setSetupPlatformMenu(false);$('[data-setup-platform-trigger]')?.focus();return}const option=event.target.closest?.('[data-setup-platform-option]');if(option&&['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();const options=$$('[data-setup-platform-option]',menu),index=options.indexOf(option),next=event.key==='Home'?0:event.key==='End'?options.length-1:(index+(event.key==='ArrowDown'?1:-1)+options.length)%options.length;options[next]?.focus();return}if(event.target.matches?.('[data-setup-platform-trigger]')&&['ArrowDown','Enter',' '].includes(event.key)){event.preventDefault();setSetupPlatformMenu(true);$('.setup-platform-menu [aria-selected="true"]')?.focus();return}if(event.key==='Escape'&&$('#modal-root').children.length)closeModal(); });
document.addEventListener('input', event => { if(event.target.matches('input[type="color"]')){event.target.closest('.color-field')?.querySelector('code')?.replaceChildren(event.target.value);const form=event.target.closest('form');if(form)applyTheme({...state.data.theme,...collectForm(form)})}if(event.target.matches('#checkout-promo'))validateCheckoutPromo(event.target); });
document.addEventListener('change',event=>{if(event.target.matches('[name="client_all_platforms"]')){const fieldset=$('.platform-picker',event.target.closest('[data-subpage-client]'));if(fieldset)fieldset.disabled=event.target.checked}if(event.target.matches('[name="client_featured"]')&&event.target.checked){$$('[name="client_featured"]',event.target.form).forEach(input=>{if(input!==event.target)input.checked=false})}});

let liveController=null,liveConnecting=false,liveReconnectTimer=null,liveBackoff=1000,liveBootstrapLoading=false,liveBootstrapQueued=false,liveMuteBootstrapUntil=0,liveMuteSupportUntil=0;
function handleLiveEvent(event) {
  if(event.type==='ready'){
    refreshBootstrapLive();
    if(state.data.user.is_admin&&(state.page==='support'||state.page.startsWith('ticket:')))loadSupportTickets();
    return;
  }
  if(event.type==='support'){
    if(Date.now()>=liveMuteSupportUntil)loadSupportTickets();
    return;
  }
  if((event.type==='account'||event.type==='bootstrap')&&Date.now()>=liveMuteBootstrapUntil)refreshBootstrapLive();
}
async function refreshBootstrapLive() {
  if(liveBootstrapLoading){liveBootstrapQueued=true;return}
  liveBootstrapLoading=true;
  const page=state.page,view=$('#page-view'),input=$('#chat-form input'),snapshot={top:view?.scrollTop||0,draft:input?.value||'',focused:document.activeElement===input};
  try{
    state.data=await api('/api/bootstrap');
    applyTheme(state.data.theme);
    render();
    requestAnimationFrame(()=>{if(state.page!==page)return;const next=$('#page-view'),nextInput=$('#chat-form input');if(next)next.scrollTop=snapshot.top;if(nextInput){nextInput.value=snapshot.draft;if(snapshot.focused)nextInput.focus()}});
  }catch(_){/* The stream will retry; ordinary API actions still surface errors. */}
  finally{liveBootstrapLoading=false;if(liveBootstrapQueued){liveBootstrapQueued=false;refreshBootstrapLive()}}
}
async function connectLiveUpdates() {
  if(state.preview||!state.token||liveConnecting||document.visibilityState==='hidden')return;
  liveConnecting=true;
  const controller=new AbortController();liveController=controller;
  try{
    const response=await fetch('/api/events',{headers:{Accept:'text/event-stream',Authorization:`Bearer ${state.token}`},cache:'no-store',signal:controller.signal});
    if(!response.ok||!response.body)throw new Error('live stream unavailable');
    liveBackoff=1000;
    const reader=response.body.getReader(),decoder=new TextDecoder();let buffer='';
    while(true){
      const {value,done}=await reader.read();if(done)break;
      buffer+=decoder.decode(value,{stream:true}).replace(/\r/g,'');
      const blocks=buffer.split('\n\n');buffer=blocks.pop()||'';
      blocks.forEach(block=>{const data=block.split('\n').filter(line=>line.startsWith('data:')).map(line=>line.slice(5).trim()).join('\n');if(!data)return;try{handleLiveEvent(JSON.parse(data))}catch(_){}});
    }
  }catch(error){if(error.name!=='AbortError')liveBackoff=Math.min(15000,Math.round(liveBackoff*1.7))}
  finally{
    if(liveController===controller)liveController=null;
    liveConnecting=false;
    if(document.visibilityState!=='hidden'){clearTimeout(liveReconnectTimer);liveReconnectTimer=setTimeout(connectLiveUpdates,liveBackoff)}
  }
}
document.addEventListener('visibilitychange',()=>{if(document.visibilityState==='hidden'){clearTimeout(liveReconnectTimer);liveController?.abort()}else connectLiveUpdates()});
window.addEventListener('pagehide',()=>liveController?.abort());

async function loadBootstrap() { state.data = await api('/api/bootstrap'); applyTheme(state.data.theme); render(); }

async function init() {
  try {
    await loadIconSprite();
    if (state.preview) { state.data = previewData(); applyTheme(state.data.theme); render(); return; }
    if (!state.token) {
      if (!tg?.initData) throw new Error('Откройте приложение кнопкой внутри Telegram-бота.');
      const auth = await api('/api/auth/telegram', {method:'POST', body:JSON.stringify({init_data:tg.initData, referral_code:tg.initDataUnsafe?.start_param || ''})});
      state.token=auth.token;sessionStorage.setItem('tgs_session',state.token);
    }
    await loadBootstrap();
    connectLiveUpdates();
    if (new URLSearchParams(location.search).get('page') === 'trial' && !state.data.user.trial_used) setTimeout(()=>document.querySelector('[data-action="trial"]')?.click(),250);
  } catch(e) { sessionStorage.removeItem('tgs_session'); $('#app').innerHTML=`<main class="locked-screen">${icon('link')}<h1>TGS VPN</h1><p>${esc(e.message)}</p></main>`; }
}

function previewData(){return {
  user:{id:1,telegram_id:6402520205,username:'bruh',remnawave_username:'tgs_6402520205',first_name:'Максим',photo_url:'',is_admin:true,trial_used:false,subscription:{status:'ACTIVE',expires_at:new Date(Date.now()+37*86400000).toISOString(),traffic_limit_bytes:107374182400,traffic_used_bytes:62277025792,device_limit:0,connected_devices:3,subscription_url:'https://example.com/sub'}},
  content:{brand:'TGS VPN',logo_url:'',start_title:'Добро пожаловать',start_text:'',trial_button:'Бесплатный период',connect_button:'Подключиться',renew_button:'Продлить',no_subscription_title:'Подписки нету',subscription_expired_title:'Подписка закончилась',buy_subscription_button:'Купить подписку',traffic_used_label:'Использовано',support_welcome:'Опишите вопрос — поддержка ответит в этом чате.',support_create_button:'Создать тикет',support_tickets_title:'Ваши тикеты',support_admin_title:'Все тикеты',support_admin_hint:'Новые сообщения появляются автоматически',support_empty:'Открытых тикетов нет',connect_title:'Установка',connect_hint:'Выберите устройство и приложение — мы проведём вас через три коротких шага.',connect_recommended:'Рекомендуем',connect_open_button:'Добавить подписку',connect_install_button:'Скачать',connect_copy_button:'Скопировать ссылку',connect_empty:'Для этого устройства клиенты пока не добавлены',connect_install_title:'Установите приложение',connect_install_hint:'Скачайте клиент для своего устройства.',connect_add_title:'Добавьте подписку',connect_add_hint:'Браузер предложит открыть выбранное приложение.',connect_use_title:'Подключитесь',connect_use_hint:'Выберите сервер в приложении и включите подключение.',connect_qr_overline:'Другой способ',connect_qr_title:'Перенести на другое устройство',connect_qr_hint:'Отсканируйте QR-код камерой или сканером внутри VPN-клиента.',emergency_message:'Сервис временно недоступен.'},
  features:{trial:true,server_status:true,devices:true,payments:true,referrals:true,promo_codes:true,support:true},trial:{enabled:true,days:3,traffic_gb:10,device_limit:1,internal_squads:[],external_squad_uuid:''},grace:{enabled:true,days:2,internal_squads:['de-main']},subpage:{include_builtins:true,clients:[]},
  theme:{template:'telegram',accent:'#2aabee',background:'#111315',surface:'#1c1f22',surface_alt:'#24282d',text:'#ffffff',muted:'#8f969e'},language:{default:'ru'},emergency:{enabled:false,message:'Сервис временно недоступен.'},more_order:['servers','devices','payments','referral'],
  tariffs:[{id:'1',name:'Старт',description:'Для одного устройства',price_rub:199,days:30,traffic_gb:100,device_limit:1,pinned:false},{id:'2',name:'Оптимальный',description:'Три месяца без забот',price_rub:499,days:90,traffic_gb:300,device_limit:3,pinned:true},{id:'3',name:'Годовой',description:'Максимальная выгода',price_rub:1490,days:365,traffic_gb:0,device_limit:5,pinned:false}],tickets:[{id:'t1',subject:'Не подключается на iPhone',status:'answered',created_at:new Date().toISOString(),updated_at:new Date().toISOString(),messages:[{text:'Не получается добавить подписку',is_admin:false,created_at:new Date(Date.now()-3600000).toISOString()},{text:'Проверьте разрешение VPN в настройках iOS.',is_admin:true,created_at:new Date().toISOString()}]}],payments:[{id:'p1',provider:'yookassa',status:'succeeded',amount_rub:499,snapshot:{name:'Оптимальный'},created_at:new Date().toISOString()}],referral:{count:4,link:'https://t.me/rwTGS_bot?start=tgs17d4b22',referral_days:7,referral_traffic_gb:10},
  payment_methods:[{id:'yookassa',name:'ЮKassa',description:'Банковская карта или СБП',enabled:true},{id:'cryptobot',name:'CryptoBot',description:'Криптовалюта через Telegram',enabled:true},{id:'lava',name:'LAVA',description:'Платёжная форма LAVA Business',enabled:true},{id:'wata',name:'WATA',description:'Карты и СБП через WATA',enabled:true},{id:'platega',name:'Platega',description:'Платёжная форма Platega',enabled:true},{id:'freekassa',name:'FreeKassa',description:'Платёжная форма FreeKassa',enabled:true},{id:'heleket',name:'Heleket',description:'Криптовалютная платёжная форма',enabled:true},{id:'pally',name:'Pally',description:'Карты и СБП через Pally',enabled:true}]
};}
async function previewApi(path,options={}) {
  await new Promise(resolve=>setTimeout(resolve,80));
  if(path==='/api/connect/handoff')return {url:'https://example.com/connect/demo/launch',fallback_url:'https://example.com/connect/demo',qr_data_url:previewQRDataURL(),expires_at:Math.floor(Date.now()/1000)+900};
  if(path==='/api/promos/validate'){const body=JSON.parse(options.body||'{}'),code=String(body.promo_code||'').trim().toUpperCase();if(!['SALE30','WELCOME'].includes(code))throw new Error('Промокод не найден');const discount=code==='SALE30'?30:10,tariff=state.data.tariffs.find(item=>item.id===body.tariff_id),base=Number(tariff?.price_rub||0);return {code,discount_percent:discount,original_price_rub:base,price_rub:Math.max(1,Math.round(base*(100-discount))/100)};}
  if(path==='/api/nodes')return [{name:'Германия',country_code:'DE',status:'online'},{name:'Нидерланды',country_code:'NL',status:'online'},{name:'Финляндия',country_code:'FI',status:'offline'}];
  if(path==='/api/devices')return [{hwid:'iphone-demo',deviceModel:'iPhone 16 Pro',platform:'iOS',userAgent:'Happ 3.1'},{hwid:'windows-demo',deviceModel:'Windows PC',platform:'Windows',userAgent:'Hiddify'}];
  if(path==='/api/admin/squads')return {internal:[{uuid:'de-main',name:'Германия · Основной'},{uuid:'nl-fast',name:'Нидерланды · Быстрый'}],external:[{uuid:'public',name:'Публичный сквад'}]};
  if(path.includes('/overview'))return {users:128,active_subscriptions:94,revenue_kopecks:18423000,diagnostics:2};
  if(path.includes('/settings/')){const key=path.split('/').pop(),integrationDefaults={remnawave:{enabled:false,url:'https://panel.example.com',token:'••••••••'},yookassa:{enabled:false,shop_id:'',secret_key:'',email:''},cryptobot:{enabled:false,token:'',testnet:false},lava:{enabled:false,shop_id:'',secret_key:'',additional_key:''},wata:{enabled:false,access_token:'',api_url:'https://api.wata.pro/api/h2h'},platega:{enabled:false,merchant_id:'',secret_key:'',api_url:'https://app.platega.io'},freekassa:{enabled:false,shop_id:'',secret_word:'',secret_word2:''},heleket:{enabled:false,merchant_id:'',api_key:'',api_url:'https://api.heleket.com'},pally:{enabled:false,shop_id:'',api_token:'',api_url:'https://pal24.pro'},notifications:{enabled:false,bot_token:'',chat_id:''}},values={language:state.data.language,emergency:state.data.emergency,features:state.data.features,trial:state.data.trial,grace:state.data.grace,subpage:state.data.subpage,theme:state.data.theme,more_order:{items:state.data.more_order},system:state.data.referral,content:state.data.content,integrations:state.cache.previewIntegrations||(state.cache.previewIntegrations=integrationDefaults)};let value=values[key]||{};if((options.method||'GET').toUpperCase()==='PUT'){const incoming=JSON.parse(options.body||'{}').value||{};value=mergeObjects(value,incoming);if(key==='more_order')state.data.more_order=value.items;else if(key==='integrations')state.cache.previewIntegrations=value;else state.data[key]=value}return {key,value,theme_templates:{telegram:{accent:'#2aabee',background:'#111315',surface:'#1c1f22',surface_alt:'#24282d'},graphite:{accent:'#ffffff',background:'#0c0c0d',surface:'#19191b',surface_alt:'#232326'},emerald:{accent:'#2fbf8f',background:'#101413',surface:'#1a211f',surface_alt:'#222c29'},sand:{accent:'#e7b55e',background:'#14120f',surface:'#211e19',surface_alt:'#2b271f'},ocean:{accent:'#5aa9e6',background:'#0d1217',surface:'#182028',surface_alt:'#222d36'},violet:{accent:'#9b8cff',background:'#111016',surface:'#1d1a25',surface_alt:'#282333'},ruby:{accent:'#e06b78',background:'#151012',surface:'#22191c',surface_alt:'#2e2226'},steel:{accent:'#8ea2b5',background:'#101214',surface:'#1a1e22',surface_alt:'#242a30'}}};}
  if(path==='/api/admin/broadcast')return state.cache['admin:broadcast']||{draft:{id:'demo',text:'Новое сообщение пользователям',source_message_id:15,status:'confirmed',buttons:[]},bot_url:'https://t.me/rwTGS_bot'};
  if(path.includes('/broadcast/draft'))return {draft:{id:'demo',text:'',source_message_id:0,status:'awaiting',buttons:[]},bot_url:'https://t.me/rwTGS_bot'};
  if(path.includes('/tariffs'))return state.data.tariffs.map(x=>({...x,active:true,internal_squads:[],external_squad_uuid:'',position:0}));
  if(path.includes('/users'))return [state.data.user];
  if(path.includes('/tickets'))return state.data.tickets.map(x=>({...x,user:state.data.user}));
  if(path.includes('/diagnostics'))return [{id:'d1',source:'remnawave',message:'Проверка подключения не пройдена',resolved:false,created_at:new Date().toISOString()}];
  if(path.includes('/promos'))return [{id:'pr1',code:'WELCOME',discount_percent:10,uses:7,max_uses:100,active:true}];
  return {ok:true,message:'Готово'};
}

function previewQRDataURL(){const cells=[];for(let y=0;y<25;y++)for(let x=0;x<25;x++){const finder=(ox,oy)=>x>=ox&&x<ox+7&&y>=oy&&y<oy+7&&(x===ox||x===ox+6||y===oy||y===oy+6||(x>=ox+2&&x<=ox+4&&y>=oy+2&&y<=oy+4));if(finder(1,1)||finder(17,1)||finder(1,17)||((x*11+y*7+x*y)%9<3&&x>0&&y>0&&x<24&&y<24))cells.push(`<rect x="${x}" y="${y}" width="1" height="1"/>`)}return `data:image/svg+xml,${encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 25 25"><rect width="25" height="25" fill="white"/><g fill="black">${cells.join('')}</g></svg>`)}`}

init();
