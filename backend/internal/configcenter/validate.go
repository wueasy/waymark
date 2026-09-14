package configcenter

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// supportedTypes 支持的配置类型，与前端配置管理页面保持一致。
var supportedTypes = map[string]bool{
	"text":       true,
	"json":       true,
	"yaml":       true,
	"properties": true,
	"xml":        true,
}

// ValidateContent 按声明的类型校验配置内容格式。
// text 类型与空白内容不做校验，其余类型格式非法时返回错误。
func ValidateContent(typ, content string) error {
	if typ == "" {
		typ = "text"
	}
	if !supportedTypes[typ] {
		return fmt.Errorf("不支持的配置类型: %s", typ)
	}
	if strings.TrimSpace(content) == "" {
		return nil
	}
	switch typ {
	case "json":
		return validateJSON(content)
	case "yaml":
		return validateYAML(content)
	case "xml":
		return validateXML(content)
	case "properties":
		return validateProperties(content)
	default:
		return nil
	}
}

func validateJSON(content string) error {
	var value any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return fmt.Errorf("JSON 格式错误: %v", err)
	}
	return nil
}

func validateYAML(content string) error {
	var value any
	if err := yaml.Unmarshal([]byte(content), &value); err != nil {
		return fmt.Errorf("YAML 格式错误: %v", err)
	}
	return nil
}

func validateXML(content string) error {
	decoder := xml.NewDecoder(strings.NewReader(content))
	for {
		if _, err := decoder.Token(); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("XML 格式错误: %v", err)
		}
	}
}

func validateProperties(content string) error {
	for index, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		if strings.HasPrefix(line, "=") || strings.HasPrefix(line, ":") {
			return fmt.Errorf("Properties 格式错误: 第 %d 行缺少键名", index+1)
		}
	}
	return nil
}
