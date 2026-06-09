export function parseTarget(raw) {
  if (/^vless:\/\//i.test(raw)) {
    return parseProxyURL(raw, 'vless');
  }
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) {
    return parseProxyURL(raw);
  }
  return parseHostPort(raw);
}

function parseProxyURL(raw, expectedScheme = '') {
  let url;
  try {
    url = new URL(raw);
  } catch {
    return parseProxyURLFallback(raw, expectedScheme);
  }

  const scheme = url.protocol.replace(':', '').toLowerCase();
  if (expectedScheme && scheme !== expectedScheme) {
    return { ok: false, message: `当前只支持识别 ${expectedScheme.toUpperCase()} 连接` };
  }
  if (!['vless', 'vmess', 'trojan', 'ss'].includes(scheme)) {
    return { ok: false, message: `暂不支持识别 ${scheme || '未知'} 协议` };
  }

  const parsed = normalizeHostPort(url.hostname, url.port);
  if (parsed.ok) {
    return parsed;
  }
  return parseProxyURLFallback(raw, expectedScheme);
}

function parseProxyURLFallback(raw, expectedScheme = '') {
  const match = raw.match(/^([a-z][a-z0-9+.-]*):\/\/(?:[^@/?#]+@)?(\[[^\]]+]|[^:/?#]+):(\d+)/i);
  if (!match) {
    return { ok: false, message: '代理连接格式不正确，无法识别' };
  }

  const scheme = match[1].toLowerCase();
  if (expectedScheme && scheme !== expectedScheme) {
    return { ok: false, message: `当前只支持识别 ${expectedScheme.toUpperCase()} 连接` };
  }
  if (!['vless', 'vmess', 'trojan', 'ss'].includes(scheme)) {
    return { ok: false, message: `暂不支持识别 ${scheme || '未知'} 协议` };
  }

  return normalizeHostPort(match[2], match[3]);
}

function parseHostPort(raw) {
  const value = raw.trim();
  if (!value) {
    return { ok: false, message: '目标地址不能为空' };
  }

  if (value.includes('://')) {
    return parseProxyURL(value);
  }

  const ipv6Match = value.match(/^\[([^\]]+)]:(\d+)$/);
  if (ipv6Match) {
    return normalizeHostPort(ipv6Match[1], ipv6Match[2]);
  }

  const lastColon = value.lastIndexOf(':');
  if (lastColon <= 0 || lastColon === value.length - 1) {
    return { ok: false, message: '未识别到端口，请粘贴完整连接或 host:port' };
  }

  const host = value.slice(0, lastColon);
  const port = value.slice(lastColon + 1);
  if (host.includes(':')) {
    return { ok: false, message: 'IPv6 地址请使用 [IPv6]:端口 格式' };
  }
  return normalizeHostPort(host, port);
}

function normalizeHostPort(host, portValue) {
  const cleanHost = String(host || '').trim();
  if (!cleanHost) {
    return { ok: false, message: '未识别到目标域名或 IP' };
  }

  const decodedHost = cleanHost.replace(/^\[|\]$/g, '');
  if (!isValidHost(decodedHost)) {
    return { ok: false, message: '目标域名或 IP 格式不正确' };
  }

  if (!portValue) {
    return { ok: false, message: '未识别到目标端口' };
  }

  const port = Number(portValue);
  if (!Number.isInteger(port) || port < 1 || port > 65535) {
    return { ok: false, message: '端口必须是 1-65535 的整数' };
  }

  return {
    ok: true,
    host: decodedHost,
    port,
  };
}

function isValidHost(host) {
  if (host.length > 253 || /\s/.test(host)) {
    return false;
  }
  if (/^(\d{1,3}\.){3}\d{1,3}$/.test(host)) {
    return host.split('.').every((item) => Number(item) >= 0 && Number(item) <= 255);
  }
  if (host.includes(':')) {
    return /^[0-9a-f:]+$/i.test(host);
  }
  return /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/i.test(host);
}
