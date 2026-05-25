<template>
  <el-card class="panel-card" shadow="never">
    <template #header>
      <div class="install-header">
        <div>
          <strong>{{ title }}</strong>
          <p class="muted install-subtitle">{{ subtitle }}</p>
        </div>
        <el-button type="primary" plain @click="handleCopy">
          一键复制
        </el-button>
      </div>
    </template>

    <div class="code-block">
      <pre>{{ command || '暂无安装命令' }}</pre>
    </div>
  </el-card>
</template>

<script setup>
import { ElMessage } from 'element-plus';

const props = defineProps({
  title: {
    type: String,
    required: true,
  },
  subtitle: {
    type: String,
    default: '',
  },
  command: {
    type: String,
    default: '',
  },
});

async function handleCopy() {
  if (!props.command) {
    ElMessage.warning('当前没有可复制的命令');
    return;
  }

  try {
    await copyText(props.command);
    ElMessage.success('安装命令已复制');
  } catch (error) {
    ElMessage.error('复制失败，请手动选中命令复制');
  }
}

async function copyText(text) {
  if (navigator.clipboard && window.isSecureContext) {
    await navigator.clipboard.writeText(text);
    return;
  }

  const textarea = document.createElement('textarea');
  textarea.value = text;
  textarea.setAttribute('readonly', '');
  textarea.style.position = 'fixed';
  textarea.style.top = '-9999px';
  textarea.style.left = '-9999px';
  document.body.appendChild(textarea);
  textarea.select();
  textarea.setSelectionRange(0, textarea.value.length);

  try {
    const ok = document.execCommand('copy');
    if (!ok) {
      throw new Error('copy command failed');
    }
  } finally {
    document.body.removeChild(textarea);
  }
}
</script>

<style scoped>
.install-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.install-subtitle {
  margin: 6px 0 0;
}
</style>
