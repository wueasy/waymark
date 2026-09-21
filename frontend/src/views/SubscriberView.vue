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

      <el-card>
        <template #header>
          <div class="card-header">
            <span>订阅列表</span>
            <div class="toolbar">
              <span class="subscriber-hint">实时展示集群内全部 SSE 订阅连接</span>
              <el-button size="small" @click="loadSubscribers">刷新</el-button>
            </div>
          </div>
        </template>

        <div class="filter-bar">
          <el-input
            v-model="filters.nodeId"
            placeholder="节点"
            clearable
            size="small"
            class="filter-item"
            @keyup.enter="applyFilter"
          />
          <el-input
            v-model="filters.groupName"
            placeholder="分组"
            clearable
            size="small"
            class="filter-item"
            @keyup.enter="applyFilter"
          />
          <el-input
            v-model="filters.keyword"
            placeholder="关键字（服务 / IP / 用户）"
            clearable
            size="small"
            class="filter-item filter-keyword"
            @keyup.enter="applyFilter"
          />
          <el-button size="small" type="primary" @click="applyFilter">查询</el-button>
          <el-button size="small" @click="resetFilter">重置</el-button>
        </div>

        <el-skeleton v-if="loading" animated class="table-skeleton">
          <template #template>
            <div v-for="i in 5" :key="i" class="table-skeleton-row">
              <el-skeleton-item variant="text" :style="{ flex: 13 }" />
              <el-skeleton-item variant="text" :style="{ flex: 18 }" />
              <el-skeleton-item variant="text" :style="{ flex: 15 }" />
              <el-skeleton-item variant="text" :style="{ flex: 18 }" />
              <el-skeleton-item variant="text" :style="{ flex: 18 }" />
              <el-skeleton-item variant="text" :style="{ flex: 14 }" />
              <el-skeleton-item variant="text" :style="{ flex: 12 }" />
              <el-skeleton-item variant="text" :style="{ flex: 15 }" />
              <el-skeleton-item variant="text" :style="{ flex: 15 }" />
            </div>
          </template>
        </el-skeleton>
        <el-table v-else :data="subscribers" size="small" empty-text="暂无订阅端连接">
          <el-table-column prop="namespace" label="命名空间" width="130" />
          <el-table-column prop="nodeId" label="节点" min-width="180" show-overflow-tooltip />
          <el-table-column prop="group" label="分组" width="150" />
          <el-table-column label="订阅配置" min-width="150">
            <template #default="{ row }">{{ formatConfigKeys(row.configKeys) }}</template>
          </el-table-column>
          <el-table-column label="订阅服务" min-width="150">
            <template #default="{ row }">{{ row.instanceKey || '（全部）' }}</template>
          </el-table-column>
          <el-table-column prop="clientIp" label="客户端 IP" width="140" />
          <el-table-column prop="username" label="用户" width="120" />
          <el-table-column label="连接时间" width="150">
            <template #default="{ row }">{{ fromNow(row.connectedAt) }}</template>
          </el-table-column>
          <el-table-column label="心跳时间" width="150">
            <template #default="{ row }">{{ fromNow(row.lastHeartbeat) }}</template>
          </el-table-column>
        </el-table>

        <el-pagination
          class="pager"
          layout="total, sizes, prev, pager, next"
          :total="total"
          :page-size="pageSize"
          :current-page="pageNum"
          :page-sizes="[10, 20, 50, 100]"
          @size-change="onSizeChange"
          @current-change="onPageChange"
        />
      </el-card>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { listSubscribers } from '../api'
import { loadNamespaces, namespaceStore, setNamespace } from '../store'
import { fromNow } from '../utils'

const namespaces = computed(() => namespaceStore.list)
const currentNamespace = computed({
  get: () => namespaceStore.current,
  set: (value) => setNamespace(value)
})
const namespace = computed(() => namespaceStore.current)

const subscribers = ref([])
const loading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)
const filters = ref({ nodeId: '', groupName: '', keyword: '' })

onMounted(async () => {
  try {
    await loadNamespaces()
  } catch (e) {
    ElMessage.error(e.message)
  }
  if (!namespaceStore.current) return
  await loadSubscribers()
})

// 切换命名空间后回到第一页重新加载。
watch(
  () => namespaceStore.current,
  async () => {
    pageNum.value = 1
    if (!namespaceStore.current) return
    await loadSubscribers()
  }
)

async function loadSubscribers() {
  loading.value = true
  try {
    const data = await listSubscribers({
      namespace: namespace.value,
      pageNum: pageNum.value,
      pageSize: pageSize.value,
      nodeId: filters.value.nodeId.trim(),
      groupName: filters.value.groupName.trim(),
      keyword: filters.value.keyword.trim()
    })
    subscribers.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function onPageChange(page) {
  pageNum.value = page
  loadSubscribers()
}

// 切换每页条数后回到第一页重新加载。
function onSizeChange(size) {
  pageSize.value = size
  pageNum.value = 1
  loadSubscribers()
}

// 应用筛选条件后回到第一页重新加载。
function applyFilter() {
  pageNum.value = 1
  loadSubscribers()
}

// 清空筛选条件后回到第一页重新加载。
function resetFilter() {
  filters.value = { nodeId: '', groupName: '', keyword: '' }
  pageNum.value = 1
  loadSubscribers()
}

// 订阅配置可能包含多个 dataId；列表为空表示未订阅配置，包含 "*" 表示订阅该分组下全部配置。
function formatConfigKeys(keys) {
  if (!keys || !keys.length) {
    return '（未订阅配置）'
  }
  if (keys.includes('*')) {
    return '（全部）'
  }
  return keys.join('、')
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
.subscriber-hint {
  color: var(--el-text-color-secondary);
  font-size: 13px;
}
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.filter-item {
  width: 180px;
}
.filter-keyword {
  width: 220px;
}
.pager {
  margin-top: 12px;
}
</style>
