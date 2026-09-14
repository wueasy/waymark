<template>
  <div>
    <el-card class="mb">
      <template #header>
        <div class="card-header">
          <span>运行模式</span>
          <div class="toolbar">
            <el-button @click="loadAll">刷新</el-button>
          </div>
        </div>
      </template>

      <el-descriptions :column="3" border size="small">
        <el-descriptions-item label="模式">
          <el-tag size="small" :type="mode === 'cluster' ? 'success' : 'info'">
            {{ mode === 'cluster' ? '集群模式' : '单机模式' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="当前节点">{{ nodeId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="节点地址">{{ address || '-' }}</el-descriptions-item>
        <el-descriptions-item label="Leader">{{ leader?.nodeId || '-' }}</el-descriptions-item>
        <el-descriptions-item label="本节点是否 Leader">
          <el-tag size="small" :type="isLeader ? 'success' : 'info'">{{ isLeader ? '是' : '否' }}</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="租约到期">{{ formatTime(leader?.leaseUntil) }}</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <el-card>
      <template #header>
        <div class="card-header">
          <span>集群节点</span>
          <div class="toolbar">
            <span class="meta">共 {{ nodes.length }} 个节点</span>
          </div>
        </div>
      </template>

      <el-skeleton v-if="loading" animated class="table-skeleton">
        <template #template>
          <div v-for="i in 5" :key="i" class="table-skeleton-row">
            <el-skeleton-item variant="text" :style="{ flex: 22 }" />
            <el-skeleton-item variant="text" :style="{ flex: 18 }" />
            <el-skeleton-item variant="text" :style="{ flex: 11 }" />
            <el-skeleton-item variant="text" :style="{ flex: 11 }" />
            <el-skeleton-item variant="text" :style="{ flex: 15 }" />
            <el-skeleton-item variant="text" :style="{ flex: 17 }" />
          </div>
        </template>
      </el-skeleton>
      <el-table v-else :data="nodes" size="small" empty-text="暂无节点">
        <el-table-column prop="nodeId" label="节点 ID" min-width="220" />
        <el-table-column prop="address" label="地址" min-width="180" />
        <el-table-column label="状态" width="110">
          <template #default="{ row }">
            <el-tag size="small" :type="row.status === 'UP' ? 'success' : 'danger'">
              {{ row.status === 'UP' ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="角色" width="110">
          <template #default="{ row }">
            <el-tag v-if="row.nodeId === leader?.nodeId" size="small" type="warning">Leader</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="最后心跳" width="150">
          <template #default="{ row }">{{ fromNow(row.lastHeartbeat) }}</template>
        </el-table-column>
        <el-table-column label="注册时间" width="170">
          <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { clusterLeader, clusterNodes } from '../api'
import { formatTime, fromNow } from '../utils'

const mode = ref('')
const nodeId = ref('')
const address = ref('')
const isLeader = ref(false)
const leader = ref(null)
const nodes = ref([])
const loading = ref(false)

onMounted(loadAll)

async function loadAll() {
  loading.value = true
  try {
    const [leaderData, nodesData] = await Promise.all([clusterLeader(), clusterNodes()])
    mode.value = leaderData.mode
    nodeId.value = leaderData.nodeId
    address.value = leaderData.address
    isLeader.value = leaderData.isLeader
    leader.value = leaderData.leader
    nodes.value = nodesData.nodes || []
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}
</script>
