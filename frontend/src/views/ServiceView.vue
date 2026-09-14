<template>
  <div>
    <el-empty
      v-if="!namespaces.length"
      description="暂无可用命名空间，请联系管理员为你的角色分配命名空间权限"
    />

    <template v-else>
      <div class="ns-bar">
        <span class="ns-bar-label">命名空间</span>
        <el-tabs v-model="currentNamespace" class="ns-tabs">
          <el-tab-pane v-for="ns in namespaces" :key="ns.namespace" :name="ns.namespace">
            <template #label>
              <span class="ns-tab-label">
                <span>{{ ns.name || ns.namespace }}</span>
                <span v-if="ns.name && ns.name !== ns.namespace" class="ns-tab-id">{{ ns.namespace }}</span>
              </span>
            </template>
          </el-tab-pane>
        </el-tabs>
      </div>

      <el-card class="mb">
        <template #header>
          <div class="card-header">
            <span>服务注册</span>
            <div class="toolbar">
              <el-input v-model="group" style="width: 180px" placeholder="分组，默认 DEFAULT_GROUP" @change="loadServices" />
              <el-button @click="loadServices">刷新</el-button>
            </div>
          </div>
        </template>

        <el-skeleton v-if="loading" animated class="table-skeleton">
          <template #template>
            <div v-for="i in 5" :key="i" class="table-skeleton-row">
              <el-skeleton-item variant="text" :style="{ flex: 22 }" />
              <el-skeleton-item variant="text" :style="{ flex: 16 }" />
              <el-skeleton-item variant="text" :style="{ flex: 10 }" />
              <el-skeleton-item variant="text" :style="{ flex: 10 }" />
              <el-skeleton-item variant="text" :style="{ flex: 12 }" />
            </div>
          </template>
        </el-skeleton>
        <el-table v-else :data="services" size="small" empty-text="暂无服务">
          <el-table-column prop="serviceName" label="服务名" min-width="220" />
          <el-table-column prop="groupName" label="分组" width="160" />
          <el-table-column label="实例数" width="100">
            <template #default="{ row }">{{ row.instanceCount }}</template>
          </el-table-column>
          <el-table-column label="健康实例" width="100">
            <template #default="{ row }">
              <el-tag :type="row.healthyCount === row.instanceCount ? 'success' : 'warning'" size="small">
                {{ row.healthyCount }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="120">
            <template #default="{ row }">
              <el-button link type="primary" @click="selectService(row)">查看实例</el-button>
            </template>
          </el-table-column>
        </el-table>

        <el-pagination
          class="pager"
          layout="total, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="pageNum"
          @current-change="onPageChange"
        />
      </el-card>

      <el-dialog
        v-model="instanceVisible"
        :title="currentService ? `实例列表 · ${currentService.serviceName}` : '实例列表'"
        width="min(80vw, 1080px)"
      >
        <div class="toolbar instance-toolbar">
          <el-button size="small" @click="loadInstances">刷新</el-button>
        </div>

        <el-skeleton v-if="instanceLoading" animated class="table-skeleton">
          <template #template>
            <div v-for="i in 5" :key="i" class="table-skeleton-row">
              <el-skeleton-item variant="text" :style="{ flex: 18 }" />
              <el-skeleton-item variant="text" :style="{ flex: 8 }" />
              <el-skeleton-item variant="text" :style="{ flex: 10 }" />
              <el-skeleton-item variant="text" :style="{ flex: 10 }" />
              <el-skeleton-item variant="text" :style="{ flex: 11 }" />
              <el-skeleton-item variant="text" :style="{ flex: 18 }" />
              <el-skeleton-item variant="text" :style="{ flex: 15 }" />
              <el-skeleton-item v-if="canWrite" variant="text" :style="{ flex: 9 }" />
            </div>
          </template>
        </el-skeleton>
        <el-table v-else :data="instances" size="small" empty-text="暂无实例">
          <el-table-column label="地址" min-width="180">
            <template #default="{ row }">{{ row.ip }}:{{ row.port }}</template>
          </el-table-column>
          <el-table-column prop="weight" label="权重" width="80" />
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="row.healthy === 1 ? 'success' : 'danger'" size="small">
                {{ row.healthy === 1 ? '健康' : '不健康' }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="类型" width="100">
            <template #default="{ row }">
              <el-tag size="small" type="info">{{ row.ephemeral === 1 ? '临时' : '持久' }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="clusterName" label="集群" width="110" />
          <el-table-column label="元数据" min-width="180">
            <template #default="{ row }">{{ metadataText(row.metadata) }}</template>
          </el-table-column>
          <el-table-column label="最后心跳" width="150">
            <template #default="{ row }">{{ fromNow(row.lastHeartbeat) }}</template>
          </el-table-column>
          <el-table-column v-if="canWrite" label="操作" width="90">
            <template #default="{ row }">
              <el-button link type="primary" @click="setHealthy(row, row.healthy === 1 ? 0 : 1)">
                {{ row.healthy === 1 ? '下线' : '上线' }}
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  listInstances,
  listServices,
  updateInstance
} from '../api'
import { loadNamespaces, namespaceStore, setNamespace, userStore } from '../store'
import { fromNow } from '../utils'

const canWrite = computed(() => {
  const p = userStore.profile
  if (!p) return false
  return !!p.isAdmin || p.permissions?.[namespaceStore.current] === 'write'
})

const namespaces = computed(() => namespaceStore.list)
const currentNamespace = computed({
  get: () => namespaceStore.current,
  set: (value) => setNamespace(value)
})
const namespace = computed(() => namespaceStore.current)
const group = ref('')
const services = ref([])
const loading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)

const currentService = ref(null)
const instances = ref([])
const instanceLoading = ref(false)
const instanceVisible = ref(false)

onMounted(async () => {
  try {
    await loadNamespaces()
  } catch (e) {
    ElMessage.error(e.message)
  }
  if (!namespaceStore.current) return
  await loadServices()
})

// 切换命名空间后，重置当前服务并重新加载。
watch(
  () => namespaceStore.current,
  async () => {
    currentService.value = null
    instances.value = []
    instanceVisible.value = false
    pageNum.value = 1
    if (!namespaceStore.current) return
    await loadServices()
  }
)

async function loadServices() {
  loading.value = true
  try {
    const data = await listServices({
      namespace: namespace.value,
      groupName: group.value,
      pageNum: pageNum.value,
      pageSize: pageSize.value
    })
    services.value = data.list || []
    total.value = data.total || 0
    if (currentService.value) {
      const stillExists = services.value.find(
        (s) => s.serviceName === currentService.value.serviceName && s.groupName === currentService.value.groupName
      )
      if (stillExists) {
        await loadInstances()
      } else {
        currentService.value = null
        instances.value = []
        instanceVisible.value = false
      }
    }
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function onPageChange(page) {
  pageNum.value = page
  loadServices()
}

async function selectService(row) {
  currentService.value = row
  instanceVisible.value = true
  await loadInstances()
}

async function loadInstances() {
  if (!currentService.value) return
  instanceLoading.value = true
  try {
    instances.value = await listInstances({
      namespace: namespace.value,
      groupName: currentService.value.groupName,
      serviceName: currentService.value.serviceName
    })
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    instanceLoading.value = false
  }
}

// 修改实例的在线状态（上/下线）。
async function setHealthy(row, healthy) {
  try {
    await updateInstance({
      namespace: namespace.value,
      groupName: row.groupName,
      serviceName: row.serviceName,
      clusterName: row.clusterName,
      ip: row.ip,
      port: row.port,
      weight: row.weight,
      healthy,
      ephemeral: row.ephemeral,
      metadata: row.metadata || {}
    })
    ElMessage.success(healthy === 1 ? '已上线' : '已下线')
    await loadInstances()
    await loadServices()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function metadataText(metadata) {
  if (!metadata) return '-'
  const keys = Object.keys(metadata)
  if (!keys.length) return '-'
  return keys.map((k) => `${k}=${metadata[k]}`).join(', ')
}
</script>

<style scoped>
.ns-bar {
  display: flex;
  align-items: center;
  background: var(--el-bg-color);
  border-radius: 4px;
  padding: 0 16px;
  margin-bottom: 16px;
}
.ns-bar-label {
  flex-shrink: 0;
  margin-right: 16px;
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.ns-tabs {
  flex: 1;
  min-width: 0;
}
.ns-tabs :deep(.el-tabs__header) {
  margin: 0;
}
.ns-tabs :deep(.el-tabs__content) {
  display: none;
}
.ns-tab-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.ns-tab-id {
  padding: 0 6px;
  border-radius: 8px;
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 18px;
}
.pager {
  margin-top: 12px;
}
.instance-toolbar {
  justify-content: flex-end;
  margin-bottom: 12px;
}
</style>
