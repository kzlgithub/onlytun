<template>
  <el-card class="panel-card stat-card" shadow="never">
    <div class="stat-head">
      <p class="stat-title">{{ title }}</p>
      <el-icon v-if="icon" class="stat-icon"><component :is="icon" /></el-icon>
    </div>
    <div class="stat-main">{{ value }}</div>
    <p class="stat-extra">{{ subtitle }}</p>
  </el-card>
</template>

<script setup>
defineProps({
  title: {
    type: String,
    required: true,
  },
  value: {
    type: String,
    required: true,
  },
  subtitle: {
    type: String,
    default: '',
  },
  icon: {
    type: [Object, Function, String],
    default: null,
  },
});
</script>

<style scoped>
.stat-card {
  min-height: 156px;
}

.stat-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.stat-title {
  margin: 0;
  color: #72829d;
  font-size: 14px;
}

.stat-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 42px;
  height: 42px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(64, 158, 255, 0.16), rgba(103, 194, 58, 0.16));
  color: #409eff;
  font-size: 20px;
}

.stat-main {
  margin-top: 28px;
  font-size: 36px;
  font-weight: 700;
  color: #132238;
}

.stat-extra {
  margin: 12px 0 0;
  color: #92a0b8;
  font-size: 13px;
}
</style>
