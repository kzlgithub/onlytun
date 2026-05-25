export function formatBytes(value = 0) {
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let num = Number(value) || 0;
  let index = 0;
  while (num >= 1024 && index < units.length - 1) {
    num /= 1024;
    index += 1;
  }
  const digits = num >= 100 || index === 0 ? 0 : 2;
  return `${num.toFixed(digits)} ${units[index]}`;
}

export function formatSpeed(value = 0) {
  return `${formatBytes(value)}/s`;
}

export function formatRelativeTime(input) {
  if (!input) {
    return '-';
  }

  const target = new Date(input).getTime();
  if (Number.isNaN(target)) {
    return '-';
  }

  const diff = Date.now() - target;
  if (diff < 60 * 1000) {
    return '刚刚';
  }

  const minutes = Math.floor(diff / (60 * 1000));
  if (minutes < 60) {
    return `${minutes} 分钟前`;
  }

  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours} 小时前`;
  }

  const days = Math.floor(hours / 24);
  return `${days} 天前`;
}

export function formatDateTime(input) {
  if (!input) {
    return '-';
  }

  const date = new Date(input);
  if (Number.isNaN(date.getTime())) {
    return '-';
  }

  return date.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export function roleLabel(role) {
  return role === 'egress' ? '出口机' : '入口机';
}

export function protocolLabel(protocol) {
  if (protocol === 'both') {
    return 'TCP+UDP';
  }
  return String(protocol || '').toUpperCase();
}
