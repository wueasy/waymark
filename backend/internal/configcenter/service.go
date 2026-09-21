package configcenter

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"waymark/internal/diff"
	"waymark/internal/store"
)

// exportMetaFile 导出包内的元数据文件名，记录每条配置的文件名与类型。
const exportMetaFile = "metadata.json"

// exportAllLimit 全量导出时的最大配置条数。
const exportAllLimit = 10000

// ErrNothingToExport 没有匹配到可导出的配置。
var ErrNothingToExport = errors.New("没有可导出的配置")

// ErrDraftConflict 草稿基于的已发布版本已被他人覆盖，需确认差异后强制发布或放弃草稿。
var ErrDraftConflict = errors.New("配置已被他人发布，请先查看差异")

// ExportItem 描述待导出配置的定位信息。
type ExportItem struct {
	GroupName string `json:"groupName"`
	DataId    string `json:"dataId"`
}

// exportMeta 导出包内单条配置的元数据。
type exportMeta struct {
	GroupName string `json:"groupName"`
	DataId    string `json:"dataId"`
	Type      string `json:"type"`
	File      string `json:"file"`
}

// ImportResult 导入结果统计，Failed 记录逐条失败原因。
type ImportResult struct {
	Imported int      `json:"imported"`
	Failed   []string `json:"failed"`
}

// Service 配置中心：配置发布、查询、删除与历史。
type Service struct {
	store *store.Store
}

// New 创建配置中心服务。
func New(st *store.Store) *Service {
	return &Service{store: st}
}

// Publish 发布或更新配置，发布前按类型校验内容格式。
func (s *Service) Publish(namespace, group, dataId, content, typ string) error {
	if typ == "" {
		typ = "text"
	}
	if err := ValidateContent(typ, content); err != nil {
		return err
	}
	return s.store.UpsertConfig(&store.ConfigItem{
		Namespace: namespace,
		GroupName: group,
		DataId:    dataId,
		Content:   content,
		Md5:       Md5(content),
		Type:      typ,
	})
}

// Get 查询配置。
func (s *Service) Get(namespace, group, dataId string) (*store.ConfigItem, error) {
	return s.store.GetConfig(namespace, group, dataId)
}

// List 分页查询配置。
func (s *Service) List(namespace, group, dataId string, pageNum, pageSize int) ([]store.ConfigItem, int64, error) {
	return s.store.ListConfigs(namespace, group, dataId, pageNum, pageSize)
}

// Delete 删除配置。
func (s *Service) Delete(namespace, group, dataId string) error {
	return s.store.DeleteConfig(namespace, group, dataId)
}

// History 分页查询配置历史。
func (s *Service) History(namespace, group, dataId string, pageNum, pageSize int) ([]store.ConfigHistory, int64, error) {
	return s.store.ListConfigHistory(namespace, group, dataId, pageNum, pageSize)
}

// Restore 将指定历史版本还原为当前配置，还原内容会重新校验并生成新的历史记录。
func (s *Service) Restore(namespace, group, dataId string, historyId int64) error {
	history, err := s.store.GetConfigHistory(historyId)
	if err != nil {
		return err
	}
	if history.Namespace != namespace || history.GroupName != group || history.DataId != dataId {
		return fmt.Errorf("历史版本与目标配置不匹配")
	}
	return s.Publish(namespace, group, dataId, history.Content, history.Type)
}

// SaveDraft 保存配置草稿。草稿仅保存编辑内容，不写变更日志，因此不会同步到下游。
// 首次创建草稿时以当前已发布版本摘要作为冲突基线（based_md5）；已有草稿时沿用原基线，
// 保证编辑期间他人在此之上的发布能被检出。
func (s *Service) SaveDraft(namespace, group, dataId, content, typ, operator string) (*store.ConfigDraft, error) {
	existing, err := s.store.GetDraft(namespace, group, dataId)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if typ == "" {
		if existing != nil {
			typ = existing.Type
		} else {
			typ = "text"
		}
	}
	if err = ValidateContent(typ, content); err != nil {
		return nil, err
	}

	basedMd5 := ""
	if existing != nil {
		basedMd5 = existing.BasedMd5
	} else if published, err := s.store.GetConfig(namespace, group, dataId); err == nil {
		basedMd5 = published.Md5
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	draft := &store.ConfigDraft{
		Namespace: namespace,
		GroupName: group,
		DataId:    dataId,
		Content:   content,
		Md5:       Md5(content),
		Type:      typ,
		BasedMd5:  basedMd5,
		Operator:  operator,
	}
	if err = s.store.SaveDraft(draft); err != nil {
		return nil, err
	}
	return draft, nil
}

// GetDraft 查询配置草稿。
func (s *Service) GetDraft(namespace, group, dataId string) (*store.ConfigDraft, error) {
	return s.store.GetDraft(namespace, group, dataId)
}

// DiscardDraft 放弃配置草稿。
func (s *Service) DiscardDraft(namespace, group, dataId string) error {
	return s.store.DeleteDraft(namespace, group, dataId)
}

// DraftDiff 草稿与已发布版本的差异预览结果。
// HasPublished 表示配置是否已发布过；Conflict 表示草稿基线与当前已发布版本不一致（已被他人发布）。
type DraftDiff struct {
	HasPublished bool        `json:"hasPublished"`
	Changed      bool        `json:"changed"`
	Conflict     bool        `json:"conflict"`
	PublishedMd5 string      `json:"publishedMd5"`
	DraftMd5     string      `json:"draftMd5"`
	Stats        diff.Stats  `json:"stats"`
	Lines        []diff.Line `json:"lines"`
}

// PreviewDraft 对比草稿与当前已发布版本，返回结构化差异与冲突标记。
func (s *Service) PreviewDraft(namespace, group, dataId string) (*DraftDiff, error) {
	draft, err := s.store.GetDraft(namespace, group, dataId)
	if err != nil {
		return nil, err
	}

	publishedContent, publishedMd5 := "", ""
	hasPublished := true
	if item, err := s.store.GetConfig(namespace, group, dataId); err == nil {
		publishedContent, publishedMd5 = item.Content, item.Md5
	} else if errors.Is(err, store.ErrNotFound) {
		hasPublished = false
	} else {
		return nil, err
	}

	result := diff.Compare(publishedContent, draft.Content)
	return &DraftDiff{
		HasPublished: hasPublished,
		Changed:      result.Changed,
		Conflict:     draft.BasedMd5 != publishedMd5,
		PublishedMd5: publishedMd5,
		DraftMd5:     draft.Md5,
		Stats:        result.Stats,
		Lines:        result.Lines,
	}, nil
}

// PublishDraft 发布草稿：将草稿内容写入正式配置并通知下游，发布成功后删除草稿。
// force 为 true 时跳过冲突校验（强制覆盖当前已发布版本）。
func (s *Service) PublishDraft(namespace, group, dataId string, force bool) error {
	draft, err := s.store.GetDraft(namespace, group, dataId)
	if err != nil {
		return err
	}

	publishedMd5 := ""
	if item, err := s.store.GetConfig(namespace, group, dataId); err == nil {
		publishedMd5 = item.Md5
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	if !force && draft.BasedMd5 != publishedMd5 {
		return ErrDraftConflict
	}
	if err = ValidateContent(draft.Type, draft.Content); err != nil {
		return err
	}

	return s.store.PublishDraft(&store.ConfigItem{
		Namespace: namespace,
		GroupName: group,
		DataId:    dataId,
		Content:   draft.Content,
		Md5:       draft.Md5,
		Type:      draft.Type,
	}, true)
}

// Export 将配置打包为 zip 字节流。
// items 非空时按指定配置导出；否则导出命名空间下符合 group/dataId 过滤条件的全部配置（空表示不过滤）。
func (s *Service) Export(namespace, group, dataId string, items []ExportItem) ([]byte, error) {
	configs := make([]store.ConfigItem, 0)
	if len(items) > 0 {
		for _, it := range items {
			item, err := s.store.GetConfig(namespace, normalizeGroup(it.GroupName), it.DataId)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					continue
				}
				return nil, err
			}
			configs = append(configs, *item)
		}
	} else {
		list, _, err := s.store.ListConfigs(namespace, group, dataId, 1, exportAllLimit)
		if err != nil {
			return nil, err
		}
		configs = list
	}
	if len(configs) == 0 {
		return nil, ErrNothingToExport
	}

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	metas := make([]exportMeta, 0, len(configs))
	for _, item := range configs {
		file := zipEntryName(item.GroupName, item.DataId)
		w, err := zw.Create(file)
		if err != nil {
			return nil, err
		}
		if _, err = w.Write([]byte(item.Content)); err != nil {
			return nil, err
		}
		metas = append(metas, exportMeta{
			GroupName: item.GroupName,
			DataId:    item.DataId,
			Type:      item.Type,
			File:      file,
		})
	}
	metaBytes, err := json.MarshalIndent(metas, "", "  ")
	if err != nil {
		return nil, err
	}
	mw, err := zw.Create(exportMetaFile)
	if err != nil {
		return nil, err
	}
	if _, err = mw.Write(metaBytes); err != nil {
		return nil, err
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Import 解析 zip 并发布配置。
// group 非空时所有配置导入到该分组，否则沿用导出包中记录的原分组。
func (s *Service) Import(namespace, group string, data []byte) (*ImportResult, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip 文件解析失败: %w", err)
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}
	metaFile, found := files[exportMetaFile]
	if !found {
		return nil, fmt.Errorf("压缩包缺少 %s，无法识别配置", exportMetaFile)
	}
	metas, err := readExportMeta(metaFile)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{Failed: make([]string, 0)}
	for _, meta := range metas {
		dataId := strings.TrimSpace(meta.DataId)
		if dataId == "" {
			result.Failed = append(result.Failed, "存在 dataId 为空的配置，已跳过")
			continue
		}
		targetGroup := group
		if targetGroup == "" {
			targetGroup = meta.GroupName
		}
		content, err := readZipContent(files[meta.File])
		if err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s/%s: %v", meta.GroupName, dataId, err))
			continue
		}
		if err = s.Publish(namespace, normalizeGroup(targetGroup), dataId, content, meta.Type); err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s/%s: %v", meta.GroupName, dataId, err))
			continue
		}
		result.Imported++
	}
	return result, nil
}

// readExportMeta 读取并解析导出包内的元数据。
func readExportMeta(f *zip.File) ([]exportMeta, error) {
	data, err := readZipContent(f)
	if err != nil {
		return nil, err
	}
	var metas []exportMeta
	if err = json.Unmarshal([]byte(data), &metas); err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", exportMetaFile, err)
	}
	if len(metas) == 0 {
		return nil, fmt.Errorf("压缩包内没有可导入的配置")
	}
	return metas, nil
}

// readZipContent 读取 zip 内文件内容。
func readZipContent(f *zip.File) (string, error) {
	if f == nil {
		return "", errors.New("压缩包内缺少对应文件")
	}
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// normalizeGroup 分组为空时回退到默认分组。
func normalizeGroup(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return store.DefaultGroup
	}
	return group
}

// zipEntryName 生成压缩包内的相对路径：分组/DataID。
func zipEntryName(group, dataId string) string {
	g := strings.Trim(strings.TrimSpace(group), "/")
	d := strings.Trim(strings.TrimSpace(dataId), "/")
	if g == "" {
		g = store.DefaultGroup
	}
	if d == "" {
		d = "config"
	}
	return g + "/" + d
}

// Md5 计算配置内容摘要，用于变更比对。
func Md5(content string) string {
	sum := md5.Sum([]byte(content))
	return hex.EncodeToString(sum[:])
}
