package note

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"mathstudy/backend/internal/platform/identifier"
)

const MaxNotes = 2000
const MaxBody = 200000

var (
	ErrInvalid  = errors.New("笔记内容无效：标题最多 200 字，正文最多 200000 字，内部链接必须指向自己的其他笔记")
	ErrNotFound = errors.New("笔记或材料不存在，或无权访问")
	ErrConflict = errors.New("笔记已在其他位置更新，请重新加载后合并修改")
	ErrLimit    = errors.New("已达到个人笔记数量上限（2000 篇）")
)

type Note struct {
	ID         string    `json:"id"`
	OwnerID    string    `json:"owner_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	MaterialID *string   `json:"material_id"`
	SourceInfo string    `json:"source_info"`
	Revision   int64     `json:"revision"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type Input struct {
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	MaterialID *string `json:"material_id"`
	SourceInfo string  `json:"source_info"`
	Revision   int64   `json:"revision"`
}

type Summary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Link struct {
	SourceID   string    `json:"source_id"`
	TargetID   string    `json:"target_id"`
	OwnerID    string    `json:"owner_id"`
	SourceType string    `json:"source_type"`
	CreatedAt  time.Time `json:"created_at"`
}

type Network struct {
	Nodes []Summary `json:"nodes"`
	Links []Link    `json:"links"`
}

type Repository interface {
	List(context.Context, string, string) ([]Summary, error)
	Get(context.Context, string, string) (Note, error)
	Save(context.Context, string, string, Input, bool) (Note, error)
	Delete(context.Context, string, string, int64) error
	Network(context.Context, string, string) (Network, error)
	ChangeLink(context.Context, string, string, string, int64, bool) (Note, error)
	SaveMaterial(context.Context, Material) (Material, error)
	GetMaterial(context.Context, string, string) (Material, error)
	ListMaterials(context.Context, string) ([]Material, error)
}

type Service struct {
	repo    Repository
	storage MaterialStorage
}

func NewService(repo Repository, storage MaterialStorage) (*Service, error) {
	if repo == nil || storage == nil {
		return nil, errors.New("note repository or storage is nil")
	}
	return &Service{repo: repo, storage: storage}, nil
}

func (s *Service) List(ctx context.Context, owner, search string) ([]Summary, error) {
	if utf8.RuneCountInString(search) > 200 {
		return nil, ErrInvalid
	}
	return s.repo.List(ctx, owner, strings.TrimSpace(search))
}
func (s *Service) Get(ctx context.Context, owner, id string) (Note, error) {
	if !ValidID(id) {
		return Note{}, ErrNotFound
	}
	return s.repo.Get(ctx, owner, id)
}
func (s *Service) Save(ctx context.Context, owner, id string, input Input) (Note, error) {
	input.Title = strings.TrimSpace(input.Title)
	if err := Validate(input); err != nil {
		return Note{}, err
	}
	create := id == ""
	if create {
		var err error
		id, err = identifier.NewUUID()
		if err != nil {
			return Note{}, err
		}
	} else if !ValidID(id) || input.Revision < 1 {
		return Note{}, ErrInvalid
	}
	for _, target := range Targets(input.Body) {
		if target == id {
			return Note{}, ErrInvalid
		}
	}
	return s.repo.Save(ctx, owner, id, input, create)
}
func (s *Service) Delete(ctx context.Context, owner, id string, revision int64) error {
	if !ValidID(id) || revision < 1 {
		return ErrInvalid
	}
	return s.repo.Delete(ctx, owner, id, revision)
}
func (s *Service) Network(ctx context.Context, owner, id string) (Network, error) {
	if id != "" && !ValidID(id) {
		return Network{}, ErrNotFound
	}
	return s.repo.Network(ctx, owner, id)
}
func (s *Service) ChangeLink(ctx context.Context, owner, source, target string, revision int64, add bool) (Note, error) {
	if !ValidID(source) || !ValidID(target) || source == target || revision < 1 {
		return Note{}, ErrInvalid
	}
	return s.repo.ChangeLink(ctx, owner, source, target, revision, add)
}

var idPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Internal links have one canonical form shared by the editor and renderer.
var linkPattern = regexp.MustCompile(`\[([^\]\r\n]*)\]\(/notes/([0-9a-f-]{36})\)`)

func ValidID(id string) bool { return idPattern.MatchString(id) }
func Validate(input Input) error {
	if strings.TrimSpace(input.Title) == "" || utf8.RuneCountInString(input.Title) > 200 || utf8.RuneCountInString(input.Body) > MaxBody || utf8.RuneCountInString(input.SourceInfo) > 1000 || !utf8.ValidString(input.Body) || strings.ContainsRune(input.Body, 0) || strings.ContainsRune(input.Title, 0) || strings.ContainsRune(input.SourceInfo, 0) {
		return ErrInvalid
	}
	if input.MaterialID != nil && !ValidID(*input.MaterialID) {
		return ErrInvalid
	}
	if len(Targets(input.Body)) > 100 {
		return ErrInvalid
	}
	return nil
}
func Targets(body string) []string {
	ids := []string{}
	seen := map[string]bool{}
	for _, bounds := range linkRanges(body) {
		match := linkPattern.FindStringSubmatch(body[bounds[0]:bounds[1]])
		if ValidID(match[2]) && !seen[match[2]] {
			ids = append(ids, match[2])
			seen[match[2]] = true
		}
	}
	return ids
}
func RemoveLink(body, target string) string {
	var result strings.Builder
	start := 0
	for _, bounds := range linkRanges(body) {
		match := linkPattern.FindStringSubmatch(body[bounds[0]:bounds[1]])
		if match[2] != target {
			continue
		}
		result.WriteString(body[start:bounds[0]])
		result.WriteString(match[1])
		start = bounds[1]
	}
	result.WriteString(body[start:])
	return result.String()
}

// Code examples, escaped brackets and Markdown images are not note links.
func linkRanges(body string) [][]int {
	ignored := make([]bool, len(body))
	fence := byte(0)
	fenceLength := 0
	offset := 0
	for _, line := range strings.SplitAfter(body, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		indent := len(line) - len(trimmed)
		run := 0
		if len(trimmed) > 0 && (trimmed[0] == '`' || trimmed[0] == '~') {
			for run < len(trimmed) && trimmed[run] == trimmed[0] {
				run++
			}
		}
		wasFenced := fence != 0
		if indent <= 3 && run >= 3 {
			if fence == 0 {
				fence = trimmed[0]
				fenceLength = run
			} else if trimmed[0] == fence && run >= fenceLength && strings.TrimSpace(trimmed[run:]) == "" {
				fence = 0
			}
		}
		if wasFenced || fence != 0 || indent >= 4 {
			for i := offset; i < offset+len(line); i++ {
				ignored[i] = true
			}
		}
		offset += len(line)
	}
	for i := 0; i < len(body); i++ {
		if ignored[i] || body[i] != '`' {
			continue
		}
		end := i + 1
		for end < len(body) && body[end] == '`' {
			end++
		}
		closing := strings.Index(body[end:], body[i:end])
		if closing >= 0 {
			stop := end + closing + (end - i)
			for j := i; j < stop; j++ {
				ignored[j] = true
			}
			i = stop - 1
		} else {
			i = end - 1
		}
	}
	result := [][]int{}
	for _, bounds := range linkPattern.FindAllStringIndex(body, -1) {
		if ignored[bounds[0]] || (bounds[0] > 0 && body[bounds[0]-1] == '!') {
			continue
		}
		escapes := 0
		for j := bounds[0] - 1; j >= 0 && body[j] == '\\'; j-- {
			escapes++
		}
		if escapes%2 == 0 {
			result = append(result, bounds)
		}
	}
	return result
}
func AppendLink(body, target, title string) string {
	for _, id := range Targets(body) {
		if id == target {
			return body
		}
	}
	title = strings.NewReplacer("&", "&#38;", "<", "&#60;", ">", "&#62;", "`", "&#96;", "\\", "&#92;", "[", "&#91;", "]", "&#93;", "\n", " ", "\r", " ").Replace(title)
	return body + "\n\n[" + title + "](/notes/" + target + ")"
}
