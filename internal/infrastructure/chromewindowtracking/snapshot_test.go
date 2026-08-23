package chromewindowtracking

import (
	"errors"
	"reflect"
	"testing"

	application "github.com/asam-masa/browser-launcher/internal/application/chromewindowtracking"
)

func TestProviderSnapshotFiltersAndSortsChromeWindows(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		handles: []uint64{30, 20, 10, 40, 50},
		metadata: map[uint64]windowMetadata{
			10: chromeMetadata(10, 100, 1000),
			20: chromeMetadata(20, 200, 2000),
			30: chromeMetadata(30, 300, 3000),
			40: chromeMetadata(40, 400, 4000),
			50: chromeMetadata(50, 500, 5000),
		},
		revalidation: map[uint64]bool{10: true, 20: true, 30: true, 40: true, 50: true},
	}
	system.metadata[20] = windowMetadata{handle: 20, processID: 200, processStartTime: 2000, executablePath: "/other/chrome", className: chromeWindowClass, visible: true}
	system.metadata[30] = windowMetadata{handle: 30, processID: 300, processStartTime: 3000, executablePath: "/chrome", className: "OtherWindow", visible: true}
	system.metadata[40] = windowMetadata{handle: 40, processID: 400, processStartTime: 4000, executablePath: "/chrome", className: chromeWindowClass, visible: false}
	system.revalidation[50] = false

	provider := Provider{executablePath: normalizeExecutablePath("/chrome"), system: system}
	got, err := provider.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	want := application.Snapshot{{Handle: 10, ProcessID: 100, ProcessStartTime: 1000}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestProviderSnapshotSkipsUnavailableWindow(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		handles: []uint64{1, 2},
		metadata: map[uint64]windowMetadata{
			1: chromeMetadata(1, 10, 100),
			2: chromeMetadata(2, 20, 200),
		},
		inspectErrors: map[uint64]error{1: errWindowUnavailable},
		revalidation:  map[uint64]bool{2: true},
	}
	provider := Provider{executablePath: normalizeExecutablePath("/chrome"), system: system}

	got, err := provider.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	want := application.Snapshot{{Handle: 2, ProcessID: 20, ProcessStartTime: 200}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestProviderSnapshotReturnsInspectionFailureWithoutPath(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		handles:       []uint64{1},
		metadata:      map[uint64]windowMetadata{},
		inspectErrors: map[uint64]error{1: errors.New("access denied")},
	}
	provider := Provider{executablePath: normalizeExecutablePath("/secret/chrome"), system: system}

	_, err := provider.Snapshot()
	if !errors.Is(err, ErrSnapshotFailed) {
		t.Fatalf("Snapshot() error = %v, want ErrSnapshotFailed", err)
	}
	if got := err.Error(); got != "chrome window snapshot failed: inspect window: access denied" {
		t.Fatalf("Snapshot() error = %q", got)
	}
}

func TestProviderSnapshotRejectsUninitializedProvider(t *testing.T) {
	t.Parallel()

	_, err := (Provider{}).Snapshot()
	if !errors.Is(err, ErrSnapshotFailed) {
		t.Fatalf("Snapshot() error = %v, want ErrSnapshotFailed", err)
	}
	if !errors.Is(err, errProviderNotInitialized) {
		t.Fatalf("Snapshot() error = %v, want errProviderNotInitialized", err)
	}
}

func TestProviderSnapshotRejectsEmptyExecutablePath(t *testing.T) {
	t.Parallel()

	provider := Provider{system: &stubWindowSystem{}}
	_, err := provider.Snapshot()
	if !errors.Is(err, ErrSnapshotFailed) {
		t.Fatalf("Snapshot() error = %v, want ErrSnapshotFailed", err)
	}
	if !errors.Is(err, errExecutablePathRequired) {
		t.Fatalf("Snapshot() error = %v, want errExecutablePathRequired", err)
	}
}

func TestProviderSnapshotSortsByStableIdentity(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		handles: []uint64{3, 1, 2},
		metadata: map[uint64]windowMetadata{
			1: chromeMetadata(1, 30, 300),
			2: chromeMetadata(2, 20, 200),
			3: chromeMetadata(3, 10, 100),
		},
		revalidation: map[uint64]bool{1: true, 2: true, 3: true},
	}
	provider := Provider{executablePath: normalizeExecutablePath("/chrome"), system: system}

	got, err := provider.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	want := application.Snapshot{
		{Handle: 1, ProcessID: 30, ProcessStartTime: 300},
		{Handle: 2, ProcessID: 20, ProcessStartTime: 200},
		{Handle: 3, ProcessID: 10, ProcessStartTime: 100},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
}

func TestProviderValidateAcceptsMatchingChromeWindow(t *testing.T) {
	t.Parallel()

	metadata := chromeMetadata(10, 100, 1000)
	system := &stubWindowSystem{
		metadata:     map[uint64]windowMetadata{10: metadata},
		revalidation: map[uint64]bool{10: true},
	}
	provider := Provider{executablePath: normalizeExecutablePath("/chrome"), system: system}

	valid, err := provider.Validate(application.Window{Handle: 10, ProcessID: 100, ProcessStartTime: 1000})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !valid {
		t.Fatal("Validate() = false, want true")
	}
}

func TestProviderValidateRejectsChangedStableIdentity(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		metadata:     map[uint64]windowMetadata{10: chromeMetadata(10, 200, 2000)},
		revalidation: map[uint64]bool{10: true},
	}
	provider := Provider{executablePath: normalizeExecutablePath("/chrome"), system: system}

	valid, err := provider.Validate(application.Window{Handle: 10, ProcessID: 100, ProcessStartTime: 1000})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if valid {
		t.Fatal("Validate() = true, want false")
	}
}

func TestProviderValidateReturnsInspectionFailureWithoutPath(t *testing.T) {
	t.Parallel()

	system := &stubWindowSystem{
		metadata:      map[uint64]windowMetadata{},
		inspectErrors: map[uint64]error{10: errors.New("access denied")},
	}
	provider := Provider{executablePath: normalizeExecutablePath("/secret/chrome"), system: system}

	_, err := provider.Validate(application.Window{Handle: 10, ProcessID: 100, ProcessStartTime: 1000})
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("Validate() error = %v, want ErrValidationFailed", err)
	}
	if got := err.Error(); got != "chrome window validation failed: inspect window: access denied" {
		t.Fatalf("Validate() error = %q", got)
	}
}

func chromeMetadata(handle uint64, processID uint32, startTime uint64) windowMetadata {
	return windowMetadata{
		handle:           handle,
		processID:        processID,
		processStartTime: startTime,
		executablePath:   "/chrome",
		className:        chromeWindowClass,
		visible:          true,
	}
}

type stubWindowSystem struct {
	handles          []uint64
	enumerateErr     error
	metadata         map[uint64]windowMetadata
	inspectErrors    map[uint64]error
	revalidation     map[uint64]bool
	revalidationErrs map[uint64]error
	enumerateCalls   int
}

func (s *stubWindowSystem) Enumerate() ([]uint64, error) {
	s.enumerateCalls++
	return s.handles, s.enumerateErr
}

func (s *stubWindowSystem) Inspect(handle uint64) (windowMetadata, error) {
	return s.metadata[handle], s.inspectErrors[handle]
}

func (s *stubWindowSystem) Revalidate(metadata windowMetadata) (bool, error) {
	return s.revalidation[metadata.handle], s.revalidationErrs[metadata.handle]
}
