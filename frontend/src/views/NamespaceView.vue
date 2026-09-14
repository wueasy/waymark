<template>
  <div>
    <el-card>
      <template #header>
        <div class="card-header">
          <span>命名空间</span>
          <div class="toolbar">
            <el-button @click="load">刷新</el-button>
            <el-button v-if="isAdmin" type="primary" @click="openCreate">新建命名空间</el-button>
          </div>
        </div>
      </template>

      <el-skeleton v-if="loading" animated class="table-skeleton">
        <template #template>
          <div v-for="i in 5" :key="i" class="table-skeleton-row">
            <el-skeleton-item variant="text" :style="{ flex: 18 }" />
            <el-skeleton-item variant="text" :style="{ flex: 18 }" />
            <el-skeleton-item variant="text" :style="{ flex: 24 }" />
            <el-skeleton-item variant="text" :style="{ flex: 17 }" />
            <el-skeleton-item v-if="isAdmin" variant="text" :style="{ flex: 15 }" />
          </div>
        </template>
      </el-skeleton>
      <el-table v-else :data="namespaces" size="small" empty-text="暂无命名空间">
        <el-table-column prop="namespace" label="标识" min-width="180" />
        <el-table-column prop="name" label="名称" min-width="180" />
        <el-table-column prop="description" label="描述" min-width="240" show-overflow-tooltip />
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
        </el-table-column>
        <el-table-column v-if="isAdmin" label="操作" width="150">
          <template #default="{ row }">
            <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
            <el-popconfirm
              v-if="row.namespace !== 'public'"
              title="确认删除该命名空间？"
              @confirm="remove(row)"
            >
              <template #reference>
                <el-button link type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑命名空间' : '新建命名空间'" width="480px">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="标识" prop="namespace">
          <el-input v-model="form.namespace" :disabled="editing" placeholder="2-64 位字母、数字、下划线或中划线" />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="form.name" placeholder="默认与标识相同" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="选填" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submit">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { createNamespace, deleteNamespace, updateNamespace } from '../api'
import { loadNamespaces, namespaceStore, userStore } from '../store'
import { formatTime } from '../utils'

const isAdmin = computed(() => !!userStore.profile?.isAdmin)

const namespaces = computed(() => namespaceStore.list)
const loading = ref(false)

const dialogVisible = ref(false)
const editing = ref(false)
const submitting = ref(false)
const formRef = ref()
const form = reactive({ namespace: '', name: '', description: '' })

const formRules = {
  namespace: [
    { required: true, message: '请输入命名空间标识', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]{2,64}$/, message: '仅支持 2-64 位字母、数字、下划线和中划线', trigger: 'blur' }
  ]
}

onMounted(load)

async function load() {
  loading.value = true
  try {
    await loadNamespaces()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = false
  form.namespace = ''
  form.name = ''
  form.description = ''
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = true
  form.namespace = row.namespace
  form.name = row.name
  form.description = row.description
  dialogVisible.value = true
}

async function submit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    if (editing.value) {
      await updateNamespace(form.namespace, { name: form.name, description: form.description })
    } else {
      await createNamespace({ namespace: form.namespace, name: form.name, description: form.description })
    }
    ElMessage.success('保存成功')
    dialogVisible.value = false
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function remove(row) {
  try {
    await deleteNamespace(row.namespace)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}
</script>
