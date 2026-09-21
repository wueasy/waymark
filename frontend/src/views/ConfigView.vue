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
            <span>配置管理</span>
            <div class="toolbar">
              <el-input v-model="group" style="width: 170px" placeholder="分组，默认 DEFAULT_GROUP" @change="loadConfigs" />
              <el-input v-model="dataId" style="width: 180px" placeholder="按 dataId 搜索" @keyup.enter="loadConfigs" />
              <el-button @click="loadConfigs">查询</el-button>
              <el-button :loading="exporting" @click="handleExport">导出</el-button>
              <el-button v-if="canWrite" @click="openImport">导入</el-button>
              <el-button v-if="canWrite" type="primary" @click="openCreate">新建配置</el-button>
            </div>
          </div>
        </template>

        <el-skeleton v-if="loading" animated class="table-skeleton">
          <template #template>
            <div v-for="i in 5" :key="i" class="table-skeleton-row">
              <el-skeleton-item variant="text" :style="{ flex: 5 }" />
              <el-skeleton-item variant="text" :style="{ flex: 20 }" />
              <el-skeleton-item variant="text" :style="{ flex: 16 }" />
              <el-skeleton-item variant="text" :style="{ flex: 11 }" />
              <el-skeleton-item variant="text" :style="{ flex: 20 }" />
              <el-skeleton-item variant="text" :style="{ flex: 17 }" />
              <el-skeleton-item variant="text" :style="{ flex: 25 }" />
            </div>
          </template>
        </el-skeleton>
        <el-table
          v-else
          ref="tableRef"
          :data="configs"
          size="small"
          empty-text="暂无配置"
          @selection-change="onSelectionChange"
        >
          <el-table-column type="selection" width="45" />
          <el-table-column prop="dataId" label="Data ID" min-width="200">
            <template #default="{ row }">
              <span>{{ row.dataId }}</span>
              <el-tooltip v-if="row.hasDraft" :content="draftTip(row)" placement="top">
                <el-tag size="small" type="warning" effect="plain" class="draft-tag">草稿</el-tag>
              </el-tooltip>
            </template>
          </el-table-column>
          <el-table-column prop="groupName" label="分组" width="160" />
          <el-table-column label="类型" width="110">
            <template #default="{ row }">
              <el-tag size="small" effect="plain">{{ typeLabel(row.type) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="md5" label="MD5" width="200" show-overflow-tooltip />
          <el-table-column label="更新时间" width="170">
            <template #default="{ row }">{{ formatTime(row.updateTime) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="340">
            <template #default="{ row }">
              <el-button link type="primary" @click="openDetail(row)">详情</el-button>
              <el-button v-if="canWrite" link type="primary" @click="openEdit(row)">编辑</el-button>
              <template v-if="row.hasDraft">
                <el-button
                  v-if="canWrite"
                  link
                  type="primary"
                  :loading="previewing && previewRow?.dataId === row.dataId"
                  @click="openPreview(row)"
                >
                  预览发布
                </el-button>
                <el-popconfirm v-if="canWrite" title="确认放弃该配置的草稿？" @confirm="discardDraft(row)">
                  <template #reference>
                    <el-button link type="danger">放弃草稿</el-button>
                  </template>
                </el-popconfirm>
              </template>
              <el-button link type="primary" @click="openHistory(row)">历史</el-button>
              <el-popconfirm v-if="canWrite" title="确认删除该配置？" @confirm="remove(row)">
                <template #reference>
                  <el-button link type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </template>
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

      <el-dialog v-model="editVisible" :title="editTitle" width="min(72vw, 1120px)" class="config-dialog">
        <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
          <el-form-item label="Data ID" prop="dataId">
            <el-input v-model="form.dataId" :disabled="editing" placeholder="例如 app.yaml" />
          </el-form-item>
          <el-form-item label="分组">
            <el-input v-model="form.groupName" :disabled="editing" placeholder="默认 DEFAULT_GROUP" />
          </el-form-item>
          <el-form-item label="类型">
            <el-radio-group v-model="form.type" :disabled="!canWrite">
              <el-radio-button v-for="t in types" :key="t" :value="t">{{ typeLabel(t) }}</el-radio-button>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="内容">
            <div class="content-field">
              <div class="content-toolbar">
                <span v-if="contentErrors.length" class="content-status is-error">
                  {{ contentErrors[0].message }}（第 {{ contentErrors[0].line }} 行）{{ contentErrors.length > 1 ? ` 等 ${contentErrors.length} 处问题` : '' }}
                </span>
                <span v-else class="content-status is-ok">格式校验通过</span>
                <el-button v-if="canWrite && form.type === 'json'" link type="primary" @click="formatNow">
                  格式化
                </el-button>
              </div>
              <CodeEditor
                v-model="form.content"
                :type="form.type"
                :readonly="!canWrite"
                height="min(54vh, 560px)"
                @errors="contentErrors = $event"
              />
            </div>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="editVisible = false">关闭</el-button>
          <el-button v-if="canWrite" type="primary" :loading="submitting" @click="submit">
            {{ editing ? '保存草稿' : '创建并发布' }}
          </el-button>
        </template>
      </el-dialog>

      <el-dialog v-model="previewVisible" title="发布预览" width="min(66vw, 980px)" class="config-dialog preview-dialog">
        <el-alert
          v-if="preview.conflict"
          type="warning"
          :closable="false"
          show-icon
          title="配置已被他人发布"
          description="草稿基于的版本已不是当前线上版本，请先核对下方差异；确认无误后可强制发布覆盖线上内容。"
        />
        <el-descriptions :column="2" border size="small" class="preview-meta">
          <el-descriptions-item label="Data ID">{{ previewRow?.dataId }}</el-descriptions-item>
          <el-descriptions-item label="分组">{{ previewRow?.groupName }}</el-descriptions-item>
          <el-descriptions-item label="变更行数">
            <span class="diff-add">+{{ preview.stats.added }}</span>
            <span class="diff-del">-{{ preview.stats.removed }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="草稿摘要">{{ preview.draftMd5 }}</el-descriptions-item>
          <el-descriptions-item label="线上摘要" :span="2">
            {{ preview.hasPublished ? preview.publishedMd5 : '未发布' }}
          </el-descriptions-item>
        </el-descriptions>

        <div class="content-toolbar">
          <span class="content-status">
            <template v-if="!preview.hasPublished">该配置尚未发布，发布后将作为首个版本生效。</template>
            <template v-else-if="!preview.changed">草稿内容与线上版本一致，无变更。</template>
            <template v-else>共 {{ preview.stats.added }} 行新增、{{ preview.stats.removed }} 行删除。</template>
          </span>
          <el-checkbox v-if="preview.changed" v-model="showUnchanged" label="显示未变更行" size="small" />
        </div>

        <div v-if="previewLines.length" class="diff-list">
          <div v-for="(line, index) in previewLines" :key="index" class="diff-line" :class="`is-${line.op}`">
            <span class="diff-no">{{ line.oldNo || '' }}</span>
            <span class="diff-no">{{ line.newNo || '' }}</span>
            <span class="diff-op">{{ diffOpLabel(line.op) }}</span>
            <span class="diff-text">{{ line.text }}</span>
          </div>
        </div>

        <template #footer>
          <el-button @click="previewVisible = false">关闭</el-button>
          <el-button v-if="canWrite" :loading="publishing" @click="discardDraft(previewRow)">放弃草稿</el-button>
          <el-button
            v-if="canWrite"
            :type="preview.conflict ? 'danger' : 'primary'"
            :loading="publishing"
            @click="publishDraft(preview.conflict)"
          >
            {{ preview.conflict ? '强制发布' : '发布' }}
          </el-button>
        </template>
      </el-dialog>

      <el-dialog v-model="historyVisible" title="配置历史" width="min(60vw, 900px)" class="config-dialog history-dialog">
        <el-table :data="history" size="small" empty-text="暂无历史" max-height="min(46vh, 420px)">
          <el-table-column prop="md5" label="MD5" min-width="260" show-overflow-tooltip />
          <el-table-column label="类型" width="90">
            <template #default="{ row }">{{ typeLabel(row.type) }}</template>
          </el-table-column>
          <el-table-column label="发布时间" width="170">
            <template #default="{ row }">{{ formatTime(row.createTime) }}</template>
          </el-table-column>
          <el-table-column label="操作" width="140">
            <template #default="{ row }">
              <el-button link type="primary" @click="viewHistoryContent(row)">查看</el-button>
              <el-popconfirm v-if="canWrite" title="确认还原到该版本？" @confirm="restore(row)">
                <template #reference>
                  <el-button link type="primary">还原</el-button>
                </template>
              </el-popconfirm>
            </template>
          </el-table-column>
        </el-table>
        <el-pagination
          class="pager"
          layout="total, sizes, prev, pager, next"
          :total="historyTotal"
          :page-size="historyPageSize"
          :current-page="historyPageNum"
          :page-sizes="[10, 20, 50, 100]"
          @size-change="onHistorySizeChange"
          @current-change="onHistoryPageChange"
        />
      </el-dialog>

      <el-dialog v-model="historyContentVisible" title="历史版本内容" width="min(64vw, 960px)" class="config-dialog detail-dialog" append-to-body>
        <div class="detail-content">
          <div class="content-toolbar">
            <span class="content-status">类型：{{ typeLabel(historyType) }}</span>
          </div>
          <CodeEditor :model-value="historyContent" :type="historyType" readonly height="min(46vh, 420px)" />
        </div>
      </el-dialog>

      <el-dialog v-model="detailVisible" title="配置详情" width="min(64vw, 960px)" class="config-dialog detail-dialog">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="Data ID">{{ detail.dataId }}</el-descriptions-item>
          <el-descriptions-item label="分组">{{ detail.groupName }}</el-descriptions-item>
          <el-descriptions-item label="类型">
            <el-tag size="small" effect="plain">{{ typeLabel(detail.type) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="更新时间">{{ formatTime(detail.updateTime) }}</el-descriptions-item>
          <el-descriptions-item label="MD5" :span="2">{{ detail.md5 }}</el-descriptions-item>
        </el-descriptions>
        <div class="detail-content">
          <div class="content-toolbar">
            <span class="content-status">配置内容</span>
          </div>
          <CodeEditor :model-value="detail.content" :type="detail.type" readonly height="min(46vh, 420px)" />
        </div>
      </el-dialog>

      <el-dialog v-model="importVisible" title="导入配置" width="min(52vw, 620px)" class="config-dialog import-dialog">
        <el-form label-width="90px">
          <el-form-item label="导入文件">
            <el-upload
              ref="uploadRef"
              class="import-upload"
              drag
              :auto-upload="false"
              :limit="1"
              accept=".zip"
              :on-change="onFileChange"
              :on-remove="onFileRemove"
              :on-exceed="onFileExceed"
            >
              <div class="upload-tip">将 zip 压缩包拖到此处，或<em>点击选择</em></div>
            </el-upload>
          </el-form-item>
          <el-form-item label="目标分组">
            <el-input v-model="importGroup" placeholder="留空则沿用导出包中的分组" />
          </el-form-item>
          <el-form-item label="说明">
            <span class="import-tip">
              导入按配置逐条发布，同名配置将被覆盖；内容格式校验失败的配置会被跳过并在结果中提示。
            </span>
          </el-form-item>
        </el-form>
        <template #footer>
          <el-button @click="importVisible = false">取消</el-button>
          <el-button type="primary" :loading="importing" @click="submitImport">开始导入</el-button>
        </template>
      </el-dialog>
    </template>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import {
  configHistory,
  deleteConfig,
  discardConfigDraft,
  exportConfigs,
  getConfig,
  getConfigDraft,
  importConfigs,
  listConfigs,
  previewConfigDraft,
  publishConfig,
  publishConfigDraft,
  restoreConfig,
  saveConfigDraft
} from '../api'
import { loadNamespaces, namespaceStore, setNamespace, userStore } from '../store'
import { formatTime } from '../utils'
import CodeEditor from '../components/CodeEditor.vue'
import { CONFIG_TYPES, CONFIG_TYPE_LABELS, formatContent, validateContent } from '../configFormat'

const types = CONFIG_TYPES

/** 类型展示名，未知类型回退为原始值。 */
function typeLabel(type) {
  return CONFIG_TYPE_LABELS[type] || type || 'Text'
}

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
const dataId = ref('')
const configs = ref([])
const loading = ref(false)
const total = ref(0)
const pageNum = ref(1)
const pageSize = ref(20)

const editVisible = ref(false)
const editing = ref(false)
const submitting = ref(false)
const formRef = ref()
const form = reactive({ dataId: '', groupName: '', type: 'text', content: '' })
const contentErrors = ref([])
const editTitle = computed(() => (editing.value ? '编辑配置' : '新建配置'))

const formRules = {
  dataId: [{ required: true, message: '请输入 Data ID', trigger: 'blur' }]
}

const detailVisible = ref(false)
const detail = reactive({ dataId: '', groupName: '', type: 'text', md5: '', updateTime: 0, content: '' })

const historyVisible = ref(false)
const history = ref([])
const historyContent = ref(null)
const historyType = ref('text')
const historyContentVisible = ref(false)
const historyRow = ref(null)
const historyPageNum = ref(1)
const historyPageSize = ref(20)
const historyTotal = ref(0)

const tableRef = ref()
const selected = ref([])
const exporting = ref(false)

const importVisible = ref(false)
const importing = ref(false)
const importGroup = ref('')
const importFile = ref(null)
const uploadRef = ref()

const previewVisible = ref(false)
const previewing = ref(false)
const publishing = ref(false)
const previewRow = ref(null)
const preview = reactive({
  hasPublished: false,
  changed: false,
  conflict: false,
  publishedMd5: '',
  draftMd5: '',
  stats: { added: 0, removed: 0 },
  lines: []
})
const showUnchanged = ref(false)
const previewLines = computed(() =>
  showUnchanged.value ? preview.lines : preview.lines.filter((line) => line.op !== 'equal')
)
const DIFF_OP_LABELS = { equal: '', add: '+', del: '-' }

onMounted(async () => {
  try {
    await loadNamespaces()
  } catch (e) {
    ElMessage.error(e.message)
  }
  if (!namespaceStore.current) return
  await loadConfigs()
})

// 切换命名空间后回到第一页重新加载。
watch(
  () => namespaceStore.current,
  async () => {
    pageNum.value = 1
    if (!namespaceStore.current) return
    await loadConfigs()
  }
)

async function loadConfigs() {
  loading.value = true
  try {
    const data = await listConfigs({
      namespace: namespace.value,
      groupName: group.value,
      dataId: dataId.value,
      pageNum: pageNum.value,
      pageSize: pageSize.value
    })
    configs.value = data.list || []
    total.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

function onPageChange(page) {
  pageNum.value = page
  loadConfigs()
}

// 切换每页条数后回到第一页重新加载。
function onSizeChange(size) {
  pageSize.value = size
  pageNum.value = 1
  loadConfigs()
}

function openCreate() {
  editing.value = false
  form.dataId = ''
  form.groupName = group.value
  form.type = 'text'
  form.content = ''
  contentErrors.value = []
  editVisible.value = true
}

/** 编辑：优先载入未发布的草稿，否则载入线上内容。 */
async function openEdit(row) {
  editing.value = true
  try {
    const base = { namespace: namespace.value, groupName: row.groupName, dataId: row.dataId }
    const draft = await getConfigDraft(base)
    const item = draft || (await getConfig(base))
    form.dataId = item.dataId
    form.groupName = item.groupName
    form.type = item.type
    form.content = item.content
    contentErrors.value = []
    editVisible.value = true
  } catch (e) {
    ElMessage.error(e.message)
  }
}

/** 只读查看配置详情。 */
async function openDetail(row) {
  try {
    const item = await getConfig({
      namespace: namespace.value,
      groupName: row.groupName,
      dataId: row.dataId
    })
    detail.dataId = item.dataId
    detail.groupName = item.groupName
    detail.type = item.type
    detail.md5 = item.md5
    detail.updateTime = item.updateTime
    detail.content = item.content
    detailVisible.value = true
  } catch (e) {
    ElMessage.error(e.message)
  }
}

/** 格式化内容（目前仅 JSON），失败时保持原样。 */
function formatNow() {
  const formatted = formatContent(form.type, form.content)
  if (formatted === null) {
    ElMessage.error('当前内容不是合法的 JSON，无法格式化')
    return
  }
  form.content = formatted
}

async function submit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return
  // 保存前按类型再做一次格式校验，避免编辑器诊断尚未刷新就提交。
  const errors = validateContent(form.type, form.content)
  if (errors.length) {
    contentErrors.value = errors
    ElMessage.error(`内容格式不正确：${errors[0].message}（第 ${errors[0].line} 行）`)
    return
  }
  submitting.value = true
  try {
    const payload = {
      namespace: namespace.value,
      groupName: form.groupName || group.value,
      dataId: form.dataId,
      content: form.content,
      type: form.type
    }
    if (editing.value) {
      await saveConfigDraft(payload)
      ElMessage.success('草稿已保存，发布后才会同步到下游')
    } else {
      await publishConfig(payload)
      ElMessage.success('发布成功')
    }
    editVisible.value = false
    await loadConfigs()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    submitting.value = false
  }
}

/** 草稿提示：展示草稿操作人与更新时间。 */
function draftTip(row) {
  const parts = ['存在未发布的草稿']
  if (row.draftOperator) parts.push(`操作人：${row.draftOperator}`)
  if (row.draftUpdateTime) parts.push(`更新时间：${formatTime(row.draftUpdateTime)}`)
  return parts.join('；')
}

/** diff 行的操作标记。 */
function diffOpLabel(op) {
  return DIFF_OP_LABELS[op] || ''
}

/** 打开发布预览，拉取草稿相对线上版本的变更。 */
async function openPreview(row) {
  previewRow.value = row
  showUnchanged.value = false
  previewing.value = true
  try {
    const data = await previewConfigDraft({
      namespace: namespace.value,
      groupName: row.groupName,
      dataId: row.dataId
    })
    preview.hasPublished = !!data.hasPublished
    preview.changed = !!data.changed
    preview.conflict = !!data.conflict
    preview.publishedMd5 = data.publishedMd5 || ''
    preview.draftMd5 = data.draftMd5 || ''
    preview.stats = data.stats || { added: 0, removed: 0 }
    preview.lines = data.lines || []
    previewVisible.value = true
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    previewing.value = false
  }
}

/** 发布草稿，force 为 true 时覆盖已被他人更新的线上版本。 */
async function publishDraft(force) {
  const row = previewRow.value
  if (!row) return
  publishing.value = true
  try {
    await publishConfigDraft({
      namespace: namespace.value,
      groupName: row.groupName,
      dataId: row.dataId,
      force: !!force
    })
    ElMessage.success('发布成功')
    previewVisible.value = false
    await loadConfigs()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    publishing.value = false
  }
}

/** 放弃草稿，草稿删除后线上内容不受影响。 */
async function discardDraft(row) {
  if (!row) return
  publishing.value = true
  try {
    await discardConfigDraft({
      namespace: namespace.value,
      groupName: row.groupName,
      dataId: row.dataId
    })
    ElMessage.success('草稿已放弃')
    previewVisible.value = false
    await loadConfigs()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    publishing.value = false
  }
}

async function remove(row) {
  try {
    await deleteConfig({ namespace: namespace.value, groupName: row.groupName, dataId: row.dataId })
    ElMessage.success('已删除')
    await loadConfigs()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

async function openHistory(row) {
  historyRow.value = row
  historyPageNum.value = 1
  historyContentVisible.value = false
  historyVisible.value = true
  await loadHistory()
}

/** 按当前页码加载历史版本列表。 */
async function loadHistory() {
  if (!historyRow.value) return
  try {
    const data = await configHistory({
      namespace: namespace.value,
      groupName: historyRow.value.groupName,
      dataId: historyRow.value.dataId,
      pageNum: historyPageNum.value,
      pageSize: historyPageSize.value
    })
    history.value = data.list || []
    historyTotal.value = data.total || 0
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function onHistoryPageChange(page) {
  historyPageNum.value = page
  loadHistory()
}

// 切换历史版本每页条数后回到第一页重新加载。
function onHistorySizeChange(size) {
  historyPageSize.value = size
  historyPageNum.value = 1
  loadHistory()
}

function viewHistoryContent(row) {
  historyType.value = row.type || 'text'
  historyContent.value = row.content
  historyContentVisible.value = true
}

/** 将历史版本还原为当前配置，成功后刷新列表与历史。 */
async function restore(row) {
  try {
    await restoreConfig({
      namespace: namespace.value,
      groupName: row.groupName,
      dataId: row.dataId,
      historyId: row.id
    })
    ElMessage.success('还原成功')
    await loadConfigs()
    historyContentVisible.value = false
    await loadHistory()
  } catch (e) {
    ElMessage.error(e.message)
  }
}

function onSelectionChange(rows) {
  selected.value = rows
}

/** 导出选中配置；未选择时按当前筛选条件导出全部。 */
async function handleExport() {
  if (!configs.value.length) {
    ElMessage.warning('当前没有可导出的配置')
    return
  }
  exporting.value = true
  try {
    await exportConfigs({
      namespace: namespace.value,
      groupName: group.value,
      dataId: dataId.value,
      items: selected.value.map((row) => ({ groupName: row.groupName, dataId: row.dataId }))
    })
    ElMessage.success('导出成功')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    exporting.value = false
  }
}

function openImport() {
  importGroup.value = ''
  importFile.value = null
  uploadRef.value?.clearFiles()
  importVisible.value = true
}

function onFileChange(file) {
  importFile.value = file.raw || null
}

function onFileRemove() {
  importFile.value = null
}

function onFileExceed() {
  ElMessage.warning('一次只能导入一个 zip 文件，请先移除已选文件')
}

/** 上传 zip 导入配置，可指定目标分组覆盖导出时的分组。 */
async function submitImport() {
  if (!importFile.value) {
    ElMessage.warning('请选择要导入的 zip 文件')
    return
  }
  const formData = new FormData()
  formData.append('namespace', namespace.value)
  formData.append('groupName', importGroup.value)
  formData.append('file', importFile.value)
  importing.value = true
  try {
    const result = await importConfigs(formData)
    const failed = result?.failed?.length || 0
    if (failed) {
      ElMessage.warning(`导入完成：成功 ${result.imported} 条，失败 ${failed} 条`)
    } else {
      ElMessage.success(`导入成功：共 ${result?.imported || 0} 条`)
    }
    importVisible.value = false
    pageNum.value = 1
    await loadConfigs()
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    importing.value = false
  }
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
.content-field {
  width: 100%;
  min-width: 0;
  overflow: hidden;
}
.content-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 6px;
  min-height: 22px;
}
.content-toolbar .el-button {
  flex-shrink: 0;
}
.content-status {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.content-status.is-error {
  color: var(--el-color-danger);
}
.content-status.is-ok {
  color: var(--el-color-success);
}
.detail-content {
  margin-top: 12px;
}
.import-upload,
.import-upload :deep(.el-upload) {
  width: 100%;
}
.import-upload :deep(.el-upload-dragger) {
  padding: 20px 12px;
}
.upload-tip {
  color: var(--el-text-color-regular);
  font-size: 13px;
}
.upload-tip em {
  color: var(--el-color-primary);
  font-style: normal;
}
.import-tip {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  line-height: 1.6;
}
.draft-tag {
  margin-left: 6px;
}
</style>

<!-- el-dialog 渲染在 body 下，需用非 scoped 样式调整 -->
<style>
.config-dialog {
  --el-dialog-margin-top: 6vh;
}
.config-dialog .el-dialog__body {
  padding-top: 8px;
}
/* 允许内容自适应收缩，避免长文本把弹窗撑宽 */
.config-dialog .el-form-item__content {
  min-width: 0;
}
/* 历史/详情弹窗内容较多时在弹窗内滚动，避免超出视口 */
.history-dialog .el-dialog__body,
.detail-dialog .el-dialog__body {
  max-height: 68vh;
  overflow-y: auto;
}
/* 发布预览弹窗 */
.preview-dialog .el-dialog__body {
  max-height: 68vh;
  overflow-y: auto;
}
.preview-meta {
  margin-bottom: 12px;
}
.diff-add {
  margin-right: 12px;
  color: var(--el-color-success);
}
.diff-del {
  color: var(--el-color-danger);
}
.diff-list {
  max-height: min(42vh, 380px);
  overflow: auto;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 4px;
  background: var(--el-bg-color);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 20px;
}
.diff-line {
  display: flex;
  align-items: flex-start;
  white-space: pre-wrap;
  word-break: break-all;
}
.diff-line.is-add {
  background: var(--el-color-success-light-9);
}
.diff-line.is-del {
  background: var(--el-color-danger-light-9);
}
.diff-line.is-equal {
  color: var(--el-text-color-secondary);
}
.diff-no {
  flex: 0 0 44px;
  padding: 0 6px;
  text-align: right;
  color: var(--el-text-color-placeholder);
  user-select: none;
}
.diff-op {
  flex: 0 0 16px;
  text-align: center;
  user-select: none;
}
.diff-text {
  flex: 1;
  min-width: 0;
  padding-right: 8px;
}
</style>
