package einoagent

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	exercisehttp "mathstudy/backend-go/internal/adapter/http/exercise"
	storageadapter "mathstudy/backend-go/internal/adapter/storage"
	answerocrapp "mathstudy/backend-go/internal/application/answerocr"
	authapp "mathstudy/backend-go/internal/application/auth"
	exerciseapp "mathstudy/backend-go/internal/application/exercise"
	uploadapp "mathstudy/backend-go/internal/application/upload"
	"mathstudy/backend-go/internal/domain/user"
)

func TestAnswerImageSubmissionAcceptsRealPNGAndJPEGOnce(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			imageData, mimeType := acceptanceImage(t, format, true)
			repo := newAcceptanceExerciseRepo()
			recorder, vision := submitAcceptanceImage(t, repo, imageData, mimeType)

			if recorder.Code != http.StatusOK {
				t.Fatalf("submit status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			var response exerciseapp.SubmitResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode submit response: %v", err)
			}
			if !response.Recorded || !response.IsCorrect || response.GradingStatus != "correct" || response.StudentAnswerLatex != "x+1" {
				t.Fatalf("submit response = %#v", response)
			}
			if vision.calls != 1 || vision.sawTransaction {
				t.Fatalf("vision calls=%d saw transaction=%t", vision.calls, vision.sawTransaction)
			}
			wantMutations := acceptanceMutationSnapshot{
				tx:                 1,
				updateSessionAfter: 1,
				attempts:           1,
				upsertDKT:          1,
				dktStates:          1,
				updateProfile:      1,
			}
			if got := repo.mutations(); got != wantMutations {
				t.Fatalf("learning writes = %#v, want %#v", got, wantMutations)
			}
			if !repo.attempts[0].IsCorrect || repo.attempts[0].StudentAnswer != "x+1" {
				t.Fatalf("attempt = %#v", repo.attempts[0])
			}
			if repo.profileUpdate.TotalExercises != 8 || repo.profileUpdate.CorrectCount != 5 {
				t.Fatalf("profile update = %#v", repo.profileUpdate)
			}
		})
	}
}

func TestAnswerImageSubmissionRejectsBlankPNGAndUnreadableJPEGWithoutLearningWrites(t *testing.T) {
	for _, format := range []string{"png", "jpeg"} {
		t.Run(format, func(t *testing.T) {
			imageData, mimeType := acceptanceImage(t, format, false)
			repo := newAcceptanceExerciseRepo()
			before := repo.mutations()

			recorder, vision := submitAcceptanceImage(t, repo, imageData, mimeType)

			if recorder.Code != http.StatusUnprocessableEntity || !strings.Contains(recorder.Body.String(), `"code":"OCR_UNREADABLE"`) {
				t.Fatalf("submit status = %d body=%s", recorder.Code, recorder.Body.String())
			}
			if vision.calls != 1 || vision.sawTransaction {
				t.Fatalf("vision calls=%d saw transaction=%t", vision.calls, vision.sawTransaction)
			}
			if after := repo.mutations(); after != before {
				t.Fatalf("unreadable image mutated learning state: before=%#v after=%#v", before, after)
			}
		})
	}
}

func TestLiveAnswerOCRRecognizesRealRasterAnswersAndRejectsUnreadableImages(t *testing.T) {
	if os.Getenv("MSP_LIVE_OCR_ACCEPTANCE") != "1" {
		t.Skip("set MSP_LIVE_OCR_ACCEPTANCE=1 with MSP_OCR_ACCEPTANCE_API_KEY and MSP_OCR_ACCEPTANCE_MODEL to run")
	}
	apiKey := strings.TrimSpace(os.Getenv("MSP_OCR_ACCEPTANCE_API_KEY"))
	modelName := strings.TrimSpace(os.Getenv("MSP_OCR_ACCEPTANCE_MODEL"))
	if apiKey == "" || modelName == "" {
		t.Fatal("live OCR acceptance requires MSP_OCR_ACCEPTANCE_API_KEY and MSP_OCR_ACCEPTANCE_MODEL")
	}
	recognizer, err := NewAnswerOCR(context.Background(), Config{
		Enabled:       true,
		BaseURL:       strings.TrimSpace(os.Getenv("MSP_OCR_ACCEPTANCE_BASE_URL")),
		APIKey:        apiKey,
		Model:         modelName,
		Timeout:       45 * time.Second,
		Temperature:   0,
		MaxTokens:     300,
		MaxIterations: 1,
	})
	if err != nil {
		t.Fatalf("create live OCR recognizer: %v", err)
	}

	answerCases := []struct {
		format string
		answer string
	}{
		{format: "png", answer: "x+1"},
		{format: "jpeg", answer: "42"},
	}
	for _, test := range answerCases {
		t.Run(test.format+"_"+test.answer, func(t *testing.T) {
			data, mimeType := acceptanceAnswerImage(t, test.format, test.answer, 0x00, 0xff)
			result, err := recognizer.Recognize(context.Background(), answerocrapp.RecognizeInput{
				Image:      answerocrapp.Image{Data: data, MIMEType: mimeType},
				AnswerType: "expression",
			})
			if err != nil {
				t.Fatalf("live OCR recognize %s: %v", test.answer, err)
			}
			if result.Status != "ok" || result.Confidence < answerocrapp.MinimumConfidence || compactMathAnswer(result.AnswerLatex) != compactMathAnswer(test.answer) {
				t.Fatalf("live OCR result for %s = %#v", test.answer, result)
			}
		})
	}

	unreadableCases := []struct {
		name       string
		format     string
		answer     string
		ink        uint8
		background uint8
	}{
		{name: "blank_png", format: "png", background: 0xff},
		{name: "low_contrast_jpeg", format: "jpeg", answer: "x+1", ink: 0xf9, background: 0xfa},
	}
	for _, test := range unreadableCases {
		t.Run(test.name, func(t *testing.T) {
			data, mimeType := acceptanceAnswerImage(t, test.format, test.answer, test.ink, test.background)
			result, err := recognizer.Recognize(context.Background(), answerocrapp.RecognizeInput{
				Image:      answerocrapp.Image{Data: data, MIMEType: mimeType},
				AnswerType: "expression",
			})
			if err != nil {
				t.Fatalf("live OCR recognize unreadable image: %v", err)
			}
			if result.Status != "unreadable" && result.Confidence >= answerocrapp.MinimumConfidence {
				t.Fatalf("live OCR unreadable result = %#v", result)
			}
		})
	}
}

func submitAcceptanceImage(t *testing.T, repo *acceptanceExerciseRepo, imageData []byte, mimeType string) (*httptest.ResponseRecorder, *acceptanceVisionModel) {
	t.Helper()
	ctx := context.Background()
	localStorage := storageadapter.NewLocalStorage(t.TempDir())
	uploadService, err := uploadapp.NewService(localStorage)
	if err != nil {
		t.Fatalf("create upload service: %v", err)
	}
	uploaded, err := uploadService.SaveImage(ctx, bytes.NewReader(imageData), uploadapp.FileMeta{
		ContentType: mimeType,
		Size:        int64(len(imageData)),
	})
	if err != nil {
		t.Fatalf("upload real %s image: %v", mimeType, err)
	}
	if uploaded.ContentType != mimeType || !strings.HasPrefix(uploaded.URL, "/uploads/images/") {
		t.Fatalf("uploaded image = %#v", uploaded)
	}

	vision := &acceptanceVisionModel{
		expectedData: imageData,
		expectedMIME: mimeType,
		repo:         repo,
	}
	ocrService, err := answerocrapp.NewService(localStorage, answerOCR{model: vision})
	if err != nil {
		t.Fatalf("create answer OCR service: %v", err)
	}
	exerciseService, err := exerciseapp.NewService(repo, nil, exerciseapp.WithAnswerOCR(ocrService))
	if err != nil {
		t.Fatalf("create exercise service: %v", err)
	}
	handler, err := exercisehttp.NewHandler(
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		exerciseService,
		acceptanceAuthenticator{},
	)
	if err != nil {
		t.Fatalf("create exercise handler: %v", err)
	}
	mux := http.NewServeMux()
	handler.Register(mux, "/api/v1/exercise")
	body, err := json.Marshal(map[string]any{
		"exercise_id":      "exercise-1",
		"answer_image_url": uploaded.URL,
	})
	if err != nil {
		t.Fatalf("marshal submit request: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/exercise/submit", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer acceptance-token")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder, vision
}

type acceptanceVisionModel struct {
	expectedData   []byte
	expectedMIME   string
	repo           *acceptanceExerciseRepo
	calls          int
	sawTransaction bool
}

func (m *acceptanceVisionModel) Generate(_ context.Context, messages []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.calls++
	m.sawTransaction = m.repo.inTx || m.repo.txCalls > 0
	if len(messages) != 2 || messages[0].Role != schema.System || messages[1].Role != schema.User {
		return nil, fmt.Errorf("unexpected OCR messages: %#v", messages)
	}
	parts := messages[1].UserInputMultiContent
	if len(parts) != 2 || parts[0].Type != schema.ChatMessagePartTypeText || !strings.Contains(parts[0].Text, "expression") || parts[1].Image == nil {
		return nil, fmt.Errorf("unexpected OCR multimodal parts: %#v", parts)
	}
	imagePart := parts[1].Image
	if imagePart.Base64Data == nil || imagePart.MIMEType != m.expectedMIME || imagePart.Detail != schema.ImageURLDetailHigh {
		return nil, fmt.Errorf("unexpected OCR image part: %#v", imagePart)
	}
	decoded, err := base64.StdEncoding.DecodeString(*imagePart.Base64Data)
	if err != nil {
		return nil, fmt.Errorf("decode OCR image base64: %w", err)
	}
	if !bytes.Equal(decoded, m.expectedData) {
		return nil, errors.New("OCR image bytes differ from uploaded bytes")
	}
	visible, err := hasVisibleAnswerStrokes(decoded, m.expectedMIME)
	if err != nil {
		return nil, err
	}
	content := `{"status":"unreadable","answer_latex":"","confidence":0.15,"reason":"low contrast"}`
	if visible {
		content = `{"status":"ok","answer_latex":"x+1","confidence":0.98,"reason":"clear final answer"}`
	}
	return &schema.Message{Role: schema.Assistant, Content: content}, nil
}

func (m *acceptanceVisionModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("stream is not used by OCR acceptance tests")
}

func hasVisibleAnswerStrokes(data []byte, expectedMIME string) (bool, error) {
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return false, fmt.Errorf("decode OCR image: %w", err)
	}
	wantFormat := strings.TrimPrefix(expectedMIME, "image/")
	if wantFormat == "jpg" {
		wantFormat = "jpeg"
	}
	if format != wantFormat {
		return false, fmt.Errorf("decoded format = %q, want %q", format, wantFormat)
	}
	minimum := uint32(0xffff)
	maximum := uint32(0)
	for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y; y++ {
		for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
			gray := color.Gray16Model.Convert(decoded.At(x, y)).(color.Gray16).Y
			if uint32(gray) < minimum {
				minimum = uint32(gray)
			}
			if uint32(gray) > maximum {
				maximum = uint32(gray)
			}
		}
	}
	return maximum-minimum >= 0x2000, nil
}

func acceptanceImage(t *testing.T, format string, visible bool) ([]byte, string) {
	t.Helper()
	answer := ""
	ink := uint8(0xff)
	background := uint8(0xff)
	if visible {
		answer = "x+1"
		ink = 0x00
	} else if format == "jpeg" {
		answer = "x+1"
		ink = 0xcf
		background = 0xd0
	}
	return acceptanceAnswerImage(t, format, answer, ink, background)
}

func acceptanceAnswerImage(t *testing.T, format string, answer string, ink uint8, background uint8) ([]byte, string) {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 180, 72))
	backgroundColor := color.RGBA{R: background, G: background, B: background, A: 0xff}
	for y := canvas.Bounds().Min.Y; y < canvas.Bounds().Max.Y; y++ {
		for x := canvas.Bounds().Min.X; x < canvas.Bounds().Max.X; x++ {
			canvas.SetRGBA(x, y, backgroundColor)
		}
	}
	if answer != "" {
		drawBitmapAnswer(canvas, answer, color.RGBA{R: ink, G: ink, B: ink, A: 0xff})
	}

	var buffer bytes.Buffer
	switch format {
	case "png":
		if err := png.Encode(&buffer, canvas); err != nil {
			t.Fatalf("encode PNG fixture: %v", err)
		}
		return buffer.Bytes(), "image/png"
	case "jpeg":
		if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 90}); err != nil {
			t.Fatalf("encode JPEG fixture: %v", err)
		}
		return buffer.Bytes(), "image/jpeg"
	default:
		t.Fatalf("unsupported fixture format %q", format)
		return nil, ""
	}
}

func drawBitmapAnswer(canvas *image.RGBA, answer string, ink color.RGBA) {
	glyphs := map[rune][]string{
		'x': {"10001", "01010", "00100", "00100", "00100", "01010", "10001"},
		'+': {"00000", "00100", "00100", "11111", "00100", "00100", "00000"},
		'1': {"00100", "01100", "00100", "00100", "00100", "00100", "01110"},
		'4': {"00010", "00110", "01010", "10010", "11111", "00010", "00010"},
		'2': {"01110", "10001", "00001", "00010", "00100", "01000", "11111"},
	}
	const scale = 7
	xOffset := 12
	for _, character := range strings.ToLower(answer) {
		glyph, ok := glyphs[character]
		if !ok {
			continue
		}
		for row, pixels := range glyph {
			for column, pixel := range pixels {
				if pixel != '1' {
					continue
				}
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						canvas.SetRGBA(xOffset+column*scale+dx, 10+row*scale+dy, ink)
					}
				}
			}
		}
		xOffset += 6 * scale
	}
}

func compactMathAnswer(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "\\(", "")
	value = strings.ReplaceAll(value, "\\)", "")
	return value
}

type acceptanceAuthenticator struct{}

func (acceptanceAuthenticator) DecodeAccessToken(token string) (authapp.Principal, bool) {
	if token != "acceptance-token" {
		return authapp.Principal{}, false
	}
	return authapp.Principal{UserID: "student-1", Role: user.RoleStudent}, true
}

type acceptanceExerciseRepo struct {
	exercise                  exerciseapp.Exercise
	session                   exerciseapp.LearningSession
	profile                   exerciseapp.StudentProfile
	inTx                      bool
	txCalls                   int
	createSessionCalls        int
	updateSessionCurrentCalls int
	updateSessionAfterCalls   int
	createGeneratedCalls      int
	createProfileCalls        int
	upsertDKTCalls            int
	updateProfileCalls        int
	attempts                  []exerciseapp.AttemptRecord
	diagnoses                 []exerciseapp.DiagnosisRecord
	dktStates                 []exerciseapp.DKTState
	profileUpdate             exerciseapp.ProfileTrackingUpdate
}

func newAcceptanceExerciseRepo() *acceptanceExerciseRepo {
	return &acceptanceExerciseRepo{
		exercise: exerciseapp.Exercise{
			ID:             "exercise-1",
			OwnerTeacherID: "teacher-1",
			Status:         "PUBLISHED",
			Title:          "OCR acceptance exercise",
			Body:           "Return x+1.",
			Difficulty:     0.25,
			ConceptIDs:     []string{"algebra"},
			Meta: map[string]any{
				"type":        "short_answer",
				"answer":      "x+1",
				"answer_type": "expression",
			},
		},
		session: exerciseapp.LearningSession{
			ID:        "session-1",
			StudentID: "student-1",
		},
		profile: exerciseapp.StudentProfile{
			MasteryVector:       map[string]float64{"algebra": 0.4},
			ErrorTendency:       map[string]float64{},
			PreferredDifficulty: 0.5,
			LearningPace:        1,
			TotalExercises:      7,
			CorrectCount:        4,
		},
	}
}

type acceptanceMutationSnapshot struct {
	tx                   int
	createSession        int
	updateSessionCurrent int
	updateSessionAfter   int
	createGenerated      int
	createProfile        int
	attempts             int
	diagnoses            int
	upsertDKT            int
	dktStates            int
	updateProfile        int
}

func (r *acceptanceExerciseRepo) mutations() acceptanceMutationSnapshot {
	return acceptanceMutationSnapshot{
		tx:                   r.txCalls,
		createSession:        r.createSessionCalls,
		updateSessionCurrent: r.updateSessionCurrentCalls,
		updateSessionAfter:   r.updateSessionAfterCalls,
		createGenerated:      r.createGeneratedCalls,
		createProfile:        r.createProfileCalls,
		attempts:             len(r.attempts),
		diagnoses:            len(r.diagnoses),
		upsertDKT:            r.upsertDKTCalls,
		dktStates:            len(r.dktStates),
		updateProfile:        r.updateProfileCalls,
	}
}

func (r *acceptanceExerciseRepo) WithTx(ctx context.Context, fn func(context.Context, exerciseapp.Repository) error) error {
	r.txCalls++
	r.inTx = true
	defer func() { r.inTx = false }()
	return fn(ctx, r)
}

func (r *acceptanceExerciseRepo) GetTeacherIDForStudent(context.Context, string) (string, bool, error) {
	return "teacher-1", true, nil
}

func (r *acceptanceExerciseRepo) GetLatestSession(context.Context, string) (exerciseapp.LearningSession, bool, error) {
	return r.session, true, nil
}

func (r *acceptanceExerciseRepo) CreateSession(_ context.Context, userID string, _ time.Time) (exerciseapp.LearningSession, error) {
	r.createSessionCalls++
	return exerciseapp.LearningSession{ID: "created-session", StudentID: userID}, nil
}

func (r *acceptanceExerciseRepo) UpdateSessionCurrentContent(context.Context, string, *string) error {
	r.updateSessionCurrentCalls++
	return nil
}

func (r *acceptanceExerciseRepo) UpdateSessionAfterSubmit(context.Context, string, string, []string) error {
	r.updateSessionAfterCalls++
	return nil
}

func (r *acceptanceExerciseRepo) GetExercise(_ context.Context, exerciseID string) (exerciseapp.Exercise, bool, error) {
	return r.exercise, exerciseID == r.exercise.ID, nil
}

func (r *acceptanceExerciseRepo) GetExerciseForUpdate(ctx context.Context, exerciseID string) (exerciseapp.Exercise, bool, error) {
	return r.GetExercise(ctx, exerciseID)
}

func (r *acceptanceExerciseRepo) GetKnowledgeConcept(context.Context, string) (exerciseapp.KnowledgeConcept, bool, error) {
	return exerciseapp.KnowledgeConcept{}, false, nil
}

func (r *acceptanceExerciseRepo) CreateGeneratedExercise(context.Context, string, exerciseapp.GeneratedQuestion, time.Time) (exerciseapp.Exercise, error) {
	r.createGeneratedCalls++
	return exerciseapp.Exercise{}, nil
}

func (r *acceptanceExerciseRepo) ListRecentContentIDs(context.Context, string, int) ([]string, error) {
	return nil, nil
}

func (r *acceptanceExerciseRepo) ListCandidateExercises(context.Context, exerciseapp.CandidateFilter) ([]exerciseapp.Exercise, error) {
	return nil, nil
}

func (r *acceptanceExerciseRepo) GetProfile(context.Context, string) (exerciseapp.StudentProfile, bool, error) {
	return r.profile, true, nil
}

func (r *acceptanceExerciseRepo) CreateProfile(context.Context, string, time.Time) (exerciseapp.StudentProfile, error) {
	r.createProfileCalls++
	return exerciseapp.StudentProfile{}, nil
}

func (r *acceptanceExerciseRepo) HasSubmittedAttempt(context.Context, string, string) (bool, error) {
	return false, nil
}

func (r *acceptanceExerciseRepo) ListDKTStates(context.Context, string, []string) (map[string]exerciseapp.DKTState, error) {
	return map[string]exerciseapp.DKTState{}, nil
}

func (r *acceptanceExerciseRepo) ListRecentInteractions(context.Context, string, int) ([]exerciseapp.LearningInteraction, error) {
	return nil, nil
}

func (r *acceptanceExerciseRepo) InsertAttempt(_ context.Context, attempt exerciseapp.AttemptRecord) error {
	r.attempts = append(r.attempts, attempt)
	return nil
}

func (r *acceptanceExerciseRepo) InsertDiagnosis(_ context.Context, diagnosis exerciseapp.DiagnosisRecord) error {
	r.diagnoses = append(r.diagnoses, diagnosis)
	return nil
}

func (r *acceptanceExerciseRepo) UpsertDKTStates(_ context.Context, states []exerciseapp.DKTState) error {
	r.upsertDKTCalls++
	r.dktStates = append(r.dktStates, states...)
	return nil
}

func (r *acceptanceExerciseRepo) UpdateProfileTracking(_ context.Context, _ string, update exerciseapp.ProfileTrackingUpdate) error {
	r.updateProfileCalls++
	r.profileUpdate = update
	return nil
}
