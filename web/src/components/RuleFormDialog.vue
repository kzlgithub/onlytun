<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑规则' : '新增规则'"
    width="760px"
    class="rule-dialog"
    destroy-on-close
    @close="emit('update:modelValue', false)"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-position="top"
      class="rule-form"
    >
      <el-form-item label="规则名称" prop="name">
        <el-input v-model="form.name" placeholder="例如：上海入口 -> 美国出口" />
      </el-form-item>

      <div class="form-grid two">
        <el-form-item label="入口机" prop="ingress_machine_id">
          <el-select v-model="form.ingress_machine_id" placeholder="请选择在线入口机">
            <el-option
              v-for="machine in ingressOptions"
              :key="machine.id"
              :label="`${machine.name} (${machine.ip || '未上报IP'})`"
              :value="machine.id"
            />
          </el-select>
        </el-form-item>

        <el-form-item label="出口机" prop="egress_machine_id">
          <el-select v-model="form.egress_machine_id" placeholder="请选择在线出口机">
            <el-option
              v-for="machine in egressOptions"
              :key="machine.id"
              :label="`${machine.name} (${machine.ip || '未上报IP'})`"
              :value="machine.id"
            />
          </el-select>
        </el-form-item>
      </div>

      <div class="form-grid target-grid">
        <el-form-item label="入口端口" prop="ingress_port">
          <el-input-number v-model="form.ingress_port" :min="1" :max="65535" controls-position="right" />
        </el-form-item>

        <el-form-item prop="target_addr">
          <template #label>
            <span class="label-with-action">
              <span>目标地址</span>
              <el-button type="primary" link class="detect-link" @click.stop.prevent="detectTarget">
                识别
              </el-button>
            </span>
          </template>
          <el-input v-model="form.target_addr" placeholder="支持 IP、域名或代理链接" />
        </el-form-item>

        <el-form-item label="目标端口" prop="target_port">
          <el-input-number v-model="form.target_port" :min="1" :max="65535" controls-position="right" />
        </el-form-item>
      </div>

      <div class="form-grid protocol-row">
        <el-form-item label="协议" prop="protocol">
          <el-radio-group v-model="form.protocol" class="protocol-group">
          <el-radio-button label="tcp">TCP</el-radio-button>
          <el-radio-button label="udp">UDP</el-radio-button>
          <el-radio-button label="both">TCP+UDP</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </div>

      <el-form-item label="流量上限（GB，0 表示无限制）" prop="traffic_limit_gb">
        <el-input-number
          v-model="form.traffic_limit_gb"
          :min="0"
          :precision="2"
          :step="10"
          controls-position="right"
          class="limit-input"
        />
      </el-form-item>

      <el-form-item label="备注" prop="remark">
        <el-input v-model="form.remark" type="textarea" :rows="3" placeholder="可选备注信息" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="dialog-footer-actions">
        <el-button @click="emit('update:modelValue', false)">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          {{ isEdit ? '保存修改' : '创建规则' }}
        </el-button>
      </div>
    </template>
  </el-dialog>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue';
import { ElMessage } from 'element-plus';
import { parseTarget } from '../utils/targetParser';

const props = defineProps({
  modelValue: {
    type: Boolean,
    default: false,
  },
  submitting: {
    type: Boolean,
    default: false,
  },
  rule: {
    type: Object,
    default: null,
  },
  ingressOptions: {
    type: Array,
    default: () => [],
  },
  egressOptions: {
    type: Array,
    default: () => [],
  },
});

const emit = defineEmits(['update:modelValue', 'submit']);
const formRef = ref(null);

const defaultForm = () => ({
  name: '',
  ingress_machine_id: '',
  ingress_port: 1,
  egress_machine_id: '',
  target_addr: '',
  target_port: 1,
  protocol: 'tcp',
  traffic_limit_gb: 0,
  remark: '',
});

const form = reactive(defaultForm());

const isEdit = computed(() => Boolean(props.rule?.id));

const rules = {
  name: [{ required: true, message: '请输入规则名称', trigger: 'blur' }],
  ingress_machine_id: [{ required: true, message: '请选择入口机', trigger: 'change' }],
  ingress_port: [{ required: true, message: '请输入入口端口', trigger: 'change' }],
  egress_machine_id: [{ required: true, message: '请选择出口机', trigger: 'change' }],
  target_addr: [{ required: true, message: '请输入目标地址', trigger: 'blur' }],
  target_port: [{ required: true, message: '请输入目标端口', trigger: 'change' }],
  protocol: [{ required: true, message: '请选择协议', trigger: 'change' }],
};

function resetForm() {
  Object.assign(form, defaultForm());
}

watch(
  () => props.modelValue,
  (visible) => {
    if (!visible) {
      return;
    }

    resetForm();
    if (props.rule) {
      Object.assign(form, {
        name: props.rule.name || '',
        ingress_machine_id: props.rule.ingress_machine_id || '',
        ingress_port: props.rule.ingress_port || 1,
        egress_machine_id: props.rule.egress_machine_id || '',
        target_addr: props.rule.target_addr || '',
        target_port: props.rule.target_port || 1,
        protocol: props.rule.protocol || 'tcp',
        traffic_limit_gb: bytesToGB(props.rule.traffic_limit_bytes || 0),
        remark: props.rule.remark || '',
      });
    }
  },
);

async function handleSubmit() {
  await formRef.value.validate();
  const payload = { ...form };
  payload.traffic_limit_bytes = gbToBytes(payload.traffic_limit_gb);
  delete payload.traffic_limit_gb;
  emit('submit', payload);
}

function gbToBytes(value) {
  return Math.round(Number(value || 0) * 1024 ** 3);
}

function bytesToGB(value) {
  return Number((Number(value || 0) / 1024 ** 3).toFixed(2));
}

function detectTarget() {
  const raw = form.target_addr.trim();
  if (!raw) {
    ElMessage.warning('请先粘贴代理连接或输入目标地址');
    return;
  }

  const parsed = parseTarget(raw);
  if (!parsed.ok) {
    ElMessage.warning(parsed.message);
    return;
  }

  form.target_addr = parsed.host;
  form.target_port = parsed.port;
  ElMessage.success(`已识别：${parsed.host}:${parsed.port}`);
}

</script>

<style scoped>
:deep(.rule-dialog .el-dialog__header) {
  padding: 22px 26px 12px;
  margin-right: 0;
}

:deep(.rule-dialog .el-dialog__title) {
  font-size: 20px;
  font-weight: 700;
  color: #18263a;
}

:deep(.rule-dialog .el-dialog__body) {
  padding: 12px 26px 10px;
}

:deep(.rule-dialog .el-dialog__footer) {
  padding: 12px 26px 22px;
}

.rule-form {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-grid {
  display: grid;
  gap: 16px;
}

.form-grid.two {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.target-grid {
  grid-template-columns: 150px minmax(0, 1fr) 150px;
  align-items: start;
}

.protocol-row {
  grid-template-columns: minmax(0, 1fr);
}

:deep(.el-form-item) {
  margin-bottom: 16px;
}

:deep(.el-form-item__label) {
  margin-bottom: 7px;
  line-height: 1.2;
  color: #52657d;
  font-weight: 600;
}

:deep(.el-select),
:deep(.el-input-number) {
  width: 100%;
}

.limit-input {
  max-width: 220px;
}

:deep(.el-input__wrapper),
:deep(.el-textarea__inner) {
  border-radius: 12px;
}

.label-with-action {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.detect-link {
  padding: 0 2px;
  height: auto;
  font-weight: 600;
  vertical-align: baseline;
}

:deep(.el-input-number .el-input__wrapper) {
  padding-left: 8px;
  padding-right: 34px;
}

.protocol-group {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

:deep(.protocol-group .el-radio-button__inner) {
  min-width: 76px;
  border: 1px solid rgba(84, 112, 150, 0.18);
  border-radius: 12px;
  box-shadow: none;
  background: #ffffff;
}

:deep(.protocol-group .el-radio-button:first-child .el-radio-button__inner),
:deep(.protocol-group .el-radio-button:last-child .el-radio-button__inner) {
  border-radius: 12px;
}

:deep(.protocol-group .el-radio-button__original-radio:checked + .el-radio-button__inner) {
  border-color: #409eff;
  background: #409eff;
  box-shadow: 0 8px 18px rgba(64, 158, 255, 0.22);
}

.dialog-footer-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

@media (max-width: 760px) {
  .form-grid.two,
  .target-grid {
    grid-template-columns: 1fr;
  }
}
</style>
