<template>
  <el-dialog
    :model-value="modelValue"
    :title="isEdit ? '编辑规则' : '新增规则'"
    width="680px"
    destroy-on-close
    @close="emit('update:modelValue', false)"
  >
    <el-form
      ref="formRef"
      :model="form"
      :rules="rules"
      label-width="110px"
    >
      <el-form-item label="规则名称" prop="name">
        <el-input v-model="form.name" placeholder="例如：上海入口 -> 美国出口" />
      </el-form-item>

      <el-form-item label="入口机" prop="ingress_machine_id">
        <el-select v-model="form.ingress_machine_id" placeholder="请选择在线入口机" style="width: 100%">
          <el-option
            v-for="machine in ingressOptions"
            :key="machine.id"
            :label="`${machine.name} (${machine.ip || '未上报IP'})`"
            :value="machine.id"
          />
        </el-select>
      </el-form-item>

      <el-form-item label="入口端口" prop="ingress_port">
        <el-input-number v-model="form.ingress_port" :min="1" :max="65535" style="width: 100%" />
      </el-form-item>

      <el-form-item label="出口机" prop="egress_machine_id">
        <el-select v-model="form.egress_machine_id" placeholder="请选择在线出口机" style="width: 100%">
          <el-option
            v-for="machine in egressOptions"
            :key="machine.id"
            :label="`${machine.name} (${machine.ip || '未上报IP'})`"
            :value="machine.id"
          />
        </el-select>
      </el-form-item>

      <el-form-item label="目标地址" prop="target_addr">
        <el-input v-model="form.target_addr" placeholder="支持 IP 或域名" />
      </el-form-item>

      <el-form-item label="目标端口" prop="target_port">
        <el-input-number v-model="form.target_port" :min="1" :max="65535" style="width: 100%" />
      </el-form-item>

      <el-form-item label="协议" prop="protocol">
        <el-radio-group v-model="form.protocol">
          <el-radio-button label="tcp">TCP</el-radio-button>
          <el-radio-button label="udp">UDP</el-radio-button>
          <el-radio-button label="both">TCP+UDP</el-radio-button>
        </el-radio-group>
      </el-form-item>

      <el-form-item label="备注" prop="remark">
        <el-input v-model="form.remark" type="textarea" :rows="4" placeholder="可选备注信息" />
      </el-form-item>
    </el-form>

    <template #footer>
      <div class="toolbar">
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
        remark: props.rule.remark || '',
      });
    }
  },
);

async function handleSubmit() {
  await formRef.value.validate();
  emit('submit', { ...form });
}
</script>
