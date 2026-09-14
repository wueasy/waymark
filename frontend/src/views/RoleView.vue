<template>
  <div>
    <el-card>
      <template #header>
        <div class="card-header">
          <span>角色管理</span>
          <div class="toolbar">
            <el-button @click="load">刷新</el-button>
            <el-button type="primary" @click="openCreate">新建角色</el-button>
          </div>
        </div>
      </template>

      <el-skeleton v-if="loading" animated class="table-skeleton">
        <template #template>
          <div v-for="i in 5" :key="i" class="table-skeleton-row">
            <el-skeleton-item variant="text" :style="{ flex: 14 }" />
            <el-skeleton-item variant="text" :style="{ flex: 14 }" />
            <el-skeleton-item variant="text" :style="{ flex: 11 }" />
            <el-skeleton-item variant="text" :style="{ flex: 24 }" />
            <el-skeleton-item variant="text" :style="{ flex: 20 }" />
            <el-skeleton-item variant="text" :style="{ flex: 9 }" />
            <el-skeleton-item variant="text" :style="{ flex: 14 }" />
          </div>
        </template>
      </el-skeleton>
      <el-table v-else :data="roles" size="small" empty-text="暂无角色">
        <el-table-column prop="code" label="角色标识" min-width="140" />
        <el-table-column prop="name" label="角色名称" min-width="140" />
        <el-table-column label="权限等级" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.permission === 'write' ? 'danger' : 'info'">
              {{ permissionLabel(row.permission) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="命名空间" min-width="240">
          <template #default="{ row }">
            <template v-if="row.builtin === 1">
              <span class="meta">全部命名空间</span>
            </template>
            <template v-else-if="row.namespaces && row.namespaces.length">
              <el-tag v-for="ns in row.namespaces" :key="ns" size="small" class="ns-tag">{{ ns }}</el-tag>
            </template>
            <span v-else class="meta">未授权</span>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" :type="row.builtin === 1 ? 'warning' : 'success'">
              {{ row.builtin === 1 ? '内置' : '自定义' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140">
          <template #default="{ row }">
            <template v-if="row.builtin !== 1">
              <el-button link type="primary" @click="openEdit(row)">编辑</el-button>
              <el-popconfirm title="确认删除该角色？" @confirm="remove(row)">
                <template #reference>
                  <el-button link type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
            <span v-else class="meta">不可修改</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑角色' : '新建角色'" width="520px">
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="角色标识" prop="code">
          <el-input v-model="form.code" :disabled="editing" placeholder="2-32 位字母、数字、下划线或中划线" />
        </el-form-item>
        <el-form-item label="角色名称" prop="name">
          <el-input v-model="form.name" placeholder="用于展示的角色名称" />
        </el-form-item>
        <el-form-item label="权限等级" prop="permission">
          <el-radio-group v-model="form.permission">
            <el-radio value="read">只读</el-radio>
            <el-radio value="write">读写</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="命名空间">
          <el-select v-model="form.namespaces" multiple style="width: 100%" placeholder="选择该角色可访问的命名空间">
            <el-option v-for="ns in namespaces" :key="ns.namespace" :label="nsLabel(ns)" :value="ns.namespace" />
          </el-select>
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
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { createRole, deleteRole, listRoles, listNamespaces, updateRole } from '../api'
import { permissionLabel } from '../utils'

const roles = ref([])
const namespaces = ref([])
const loading = ref(false)

const dialogVisible = ref(false)
const editing = ref(false)
const submitting = ref(false)
const formRef = ref()
const form = reactive({ id: 0, code: '', name: '', description: '', permission: 'read', namespaces: [] })

const formRules = {
  code: [
    { required: true, message: '请输入角色标识', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_-]{2,32}$/, message: '仅支持 2-32 位字母、数字、下划线和中划线', trigger: 'blur' }
  ],
  name: [{ required: true, message: '请输入角色名称', trigger: 'blur' }],
  permission: [{ required: true, message: '请选择权限等级', trigger: 'change' }]
}

onMounted(load)

async function load() {
  loading.value = true
  try {
    const [roleList, nsList] = await Promise.all([listRoles(), listNamespaces()])
    roles.value = roleList
    namespaces.value = nsList
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

/** 命名空间展示文案：名称与标识不同时同时展示标识。 */
function nsLabel(ns) {
  return ns.name && ns.name !== ns.namespace ? `${ns.name}（${ns.namespace}）` : ns.namespace
}

function openCreate() {
  editing.value = false
  form.id = 0
  form.code = ''
  form.name = ''
  form.description = ''
  form.permission = 'read'
  form.namespaces = []
  dialogVisible.value = true
}

function openEdit(row) {
  editing.value = true
  form.id = row.id
  form.code = row.code
  form.name = row.name
  form.description = row.description
  form.permission = row.permission
  form.namespaces = [...(row.namespaces || [])]
  dialogVisible.value = true
}

async function submit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  submitting.value = true
  try {
    const payload = {
      code: form.code,
      name: form.name,
      description: form.description,
      permission: form.permission,
      namespaces: form.namespaces
    }
    if (editing.value) {
      await updateRole(form.id, payload)
    } else {
      await createRole(payload)
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
    await deleteRole(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    ElMessage.error(e.message)
  }
}
</script>

<style scoped>
.ns-tag {
  margin-right: 6px;
}
</style>
