package domain

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestValidateFunnyNameFormat_ValidNames(t *testing.T) {
	validNames := []string{"Mickey", "Donald", "BARNACLE", "Pickles", "Scooby", "Jerry", "Tom", "Bugs", "Daffy"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFunnyNameFormat(name); err != nil {
				t.Errorf("ValidateFunnyNameFormat(%q) returned error: %v", name, err)
			}
		})
	}
}

func TestValidateFunnyNameFormat_InvalidNames(t *testing.T) {
	invalidCases := []struct {
		name    string
		errType error
	}{
		{"", ErrInvalidFunnyName},
		{"Mickey123", ErrInvalidFunnyName},
		{"123Bugs", ErrInvalidFunnyName},
		{"Mickey-1", ErrInvalidFunnyName},
		{"Tom & Jerry", ErrInvalidFunnyName},
		{"Scooby!", ErrInvalidFunnyName},
		{"Daffy?", ErrInvalidFunnyName},
		{"Mickey ", ErrInvalidFunnyName},
		{" Mickey", ErrInvalidFunnyName},
		{"a", ErrFunnyNameTooShort},
		{"ab", ErrFunnyNameTooShort},
		{"THISNAMEISWAYTOOLONGTOBEVALID", ErrFunnyNameTooLong},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFunnyNameFormat(tc.name)
			if err == nil {
				t.Errorf("ValidateFunnyNameFormat(%q) = nil, expected error", tc.name)
				return
			}
			if !errors.Is(err, tc.errType) {
				t.Errorf("ValidateFunnyNameFormat(%q) = %v, expected %v", tc.name, err, tc.errType)
			}
		})
	}
}

func TestValidateFunnyNameFormat_UnderscoresAllowed(t *testing.T) {
	validUnderscoreNames := []string{"Mickey_Mouse", "Tom_Jerry", "Scooby_Doo", "Road_Runner"}
	for _, name := range validUnderscoreNames {
		t.Run(name, func(t *testing.T) {
			if err := ValidateFunnyNameFormat(name); err != nil {
				t.Errorf("ValidateFunnyNameFormat(%q) returned error: %v", name, err)
			}
		})
	}
}

func TestIsValidFunnyName_FromList(t *testing.T) {
	knownNames := []string{"Mickey", "Donald", "BARNACLE", "Pickles", "Scooby", "Jerry", "Tom", "Bugs", "Daffy", "Goofy"}
	for _, name := range knownNames {
		t.Run(name, func(t *testing.T) {
			if !IsValidFunnyName(name) {
				t.Errorf("IsValidFunnyName(%q) = false, expected true", name)
			}
		})
	}
}

func TestIsValidFunnyName_NotInList(t *testing.T) {
	notInList := []string{"Basuruk", "OmniView", "OmniInspect", "Random", "Xyz", "ABCDEFGHIJKLMNOP", "TestName", " Mickey", "Mickey "}
	for _, name := range notInList {
		t.Run(name, func(t *testing.T) {
			if IsValidFunnyName(name) {
				t.Errorf("IsValidFunnyName(%q) = true, expected false (not in curated list)", name)
			}
		})
	}
}

func TestIsValidFunnyName_CaseInsensitive(t *testing.T) {
	variations := []struct {
		name  string
		valid bool
	}{
		{"mickey", true},
		{"MICKEY", true},
		{"MiCkEy", true},
		{"BARNACLE", true},
		{"barnacle", true},
		{"Barnacle", true},
	}

	for _, v := range variations {
		t.Run(v.name, func(t *testing.T) {
			result := IsValidFunnyName(v.name)
			if result != v.valid {
				t.Errorf("IsValidFunnyName(%q) = %v, expected %v", v.name, result, v.valid)
			}
		})
	}
}

func TestFunnyNameGenerator_GetRandomName(t *testing.T) {
	gen := NewFunnyNameGenerator(42)

	name1, err := gen.GetRandomName()
	if err != nil {
		t.Fatalf("GetRandomName() returned error: %v", err)
	}
	if name1 == "" {
		t.Error("GetRandomName() returned empty string")
	}
	if !IsValidFunnyName(name1) {
		t.Errorf("GetRandomName() returned invalid funny name: %q", name1)
	}
	if name1 != strings.ToUpper(name1) {
		t.Errorf("GetRandomName() returned %q, expected uppercase procedure-safe name", name1)
	}
}

func TestFunnyNameGenerator_NoDuplicates(t *testing.T) {
	gen := NewFunnyNameGenerator(42)
	seen := make(map[string]bool)

	initialCount := gen.AvailableCount()
	if initialCount == 0 {
		t.Fatal("Generator has no available names")
	}

	for i := 0; i < initialCount; i++ {
		name, err := gen.GetRandomName()
		if err != nil {
			t.Fatalf("GetRandomName() iteration %d returned error: %v", i, err)
		}
		if seen[name] {
			t.Errorf("GetRandomName() returned duplicate name %q", name)
		}
		seen[name] = true
	}

	finalCount := gen.AvailableCount()
	if finalCount != 0 {
		t.Errorf("After exhausting all names, AvailableCount() = %d, expected 0", finalCount)
	}

	_, err := gen.GetRandomName()
	if !errors.Is(err, ErrNoAvailableNames) {
		t.Errorf("GetRandomName() after exhaustion = %v, expected ErrNoAvailableNames", err)
	}
}

func TestFunnyNameGenerator_MarkAsUsed(t *testing.T) {
	gen := NewFunnyNameGenerator(99)

	name, err := gen.GetRandomName()
	if err != nil {
		t.Fatalf("GetRandomName() returned error: %v", err)
	}
	if !gen.IsUsed(name) {
		t.Errorf("After GetRandomName() returned %q, IsUsed(%q) = false, expected true", name, name)
	}

	if err := gen.MarkAsAvailable(name); err != nil {
		t.Fatalf("MarkAsAvailable(%q) returned error: %v", name, err)
	}
	if gen.IsUsed(name) {
		t.Errorf("After MarkAsAvailable(%q), IsUsed(%q) = true, expected false", name, name)
	}
}

func TestFunnyNameGenerator_MarkAsAvailable(t *testing.T) {
	gen := NewFunnyNameGenerator(99)

	initialCount := gen.AvailableCount()
	name, err := gen.GetRandomName()
	if err != nil {
		t.Fatalf("GetRandomName() returned error: %v", err)
	}

	afterCount := gen.AvailableCount()
	if afterCount != initialCount-1 {
		t.Errorf("After GetRandomName(), AvailableCount() = %d, expected %d", afterCount, initialCount-1)
	}

	if err := gen.MarkAsAvailable(name); err != nil {
		t.Fatalf("MarkAsAvailable(%q) returned error: %v", name, err)
	}
	restoredCount := gen.AvailableCount()
	if restoredCount != initialCount {
		t.Errorf("After MarkAsAvailable(), AvailableCount() = %d, expected %d", restoredCount, initialCount)
	}

	_, err = gen.GetRandomName()
	if err != nil {
		t.Errorf("GetRandomName() after MarkAsAvailable() returned error: %v", err)
	}
}

func TestFunnyNameGenerator_Reset(t *testing.T) {
	gen := NewFunnyNameGenerator(99)

	initialCount := gen.AvailableCount()
	for i := 0; i < 3; i++ {
		if _, err := gen.GetRandomName(); err != nil {
			t.Fatalf("GetRandomName() iteration %d returned error: %v", i, err)
		}
	}

	if gen.AvailableCount() != initialCount-3 {
		t.Errorf("After 3 GetRandomName() calls, AvailableCount() = %d, expected %d", gen.AvailableCount(), initialCount-3)
	}

	gen.Reset()
	if gen.AvailableCount() != initialCount {
		t.Errorf("After Reset(), AvailableCount() = %d, expected %d", gen.AvailableCount(), initialCount)
	}
}

func TestFunnyNameGenerator_ThreadSafety(t *testing.T) {
	gen := NewFunnyNameGenerator(42)
	var wg sync.WaitGroup
	results := make(chan string, 100)
	errCh := make(chan error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name, err := gen.GetRandomName()
			if err != nil {
				errCh <- err
				return
			}
			results <- name
		}()
	}

	wg.Wait()
	close(results)
	close(errCh)

	if err, ok := <-errCh; ok {
		t.Fatalf("GetRandomName() in concurrent worker returned error: %v", err)
	}

	seen := make(map[string]bool)
	for name := range results {
		if seen[name] {
			t.Errorf("Thread safety violation: duplicate name %q returned", name)
		}
		seen[name] = true
	}
}

func TestFunnyNameGenerator_DeterministicWithSameSeed(t *testing.T) {
	gen1 := NewFunnyNameGenerator(12345)
	gen2 := NewFunnyNameGenerator(12345)

	for i := 0; i < 10; i++ {
		name1, err := gen1.GetRandomName()
		if err != nil {
			t.Fatalf("gen1.GetRandomName() iteration %d returned error: %v", i, err)
		}
		name2, err := gen2.GetRandomName()
		if err != nil {
			t.Fatalf("gen2.GetRandomName() iteration %d returned error: %v", i, err)
		}
		if name1 != name2 {
			t.Errorf("Generators with same seed produced different names at iteration %d: %q vs %q", i, name1, name2)
		}
	}
}

func TestFunnyName_Interface(t *testing.T) {
	fn, err := NewFunnyName("Mickey")
	if err != nil {
		t.Fatalf("NewFunnyName() returned error: %v", err)
	}

	if fn.Name() != "Mickey" {
		t.Errorf("Name() = %q, expected %q", fn.Name(), "Mickey")
	}

	if fn.String() != "Mickey" {
		t.Errorf("String() = %q, expected %q", fn.String(), "Mickey")
	}

	if !fn.IsValid() {
		t.Error("IsValid() = false, expected true for non-empty name with length >= 3")
	}

	var zeroFn FunnyName
	if zeroFn.IsValid() {
		t.Error("IsValid() = true for zero-value FunnyName, expected false")
	}

	if _, err := NewFunnyName(""); !errors.Is(err, ErrInvalidFunnyName) {
		t.Errorf("NewFunnyName(\"\") error = %v, expected ErrInvalidFunnyName", err)
	}

	if _, err := NewFunnyName("AB"); !errors.Is(err, ErrFunnyNameTooShort) {
		t.Errorf("NewFunnyName(\"AB\") error = %v, expected ErrFunnyNameTooShort", err)
	}

	if _, err := NewFunnyName("Mickey123"); !errors.Is(err, ErrInvalidFunnyName) {
		t.Errorf("NewFunnyName(\"Mickey123\") error = %v, expected ErrInvalidFunnyName", err)
	}
}

func TestFunnyNameGenerator_CollisionHandling(t *testing.T) {
	gen := NewFunnyNameGenerator(42)

	assigned := make(map[string]bool)
	initialCount := gen.AvailableCount()
	for i := 0; i < initialCount; i++ {
		name, err := gen.GetRandomName()
		if err != nil {
			t.Fatalf("GetRandomName() iteration %d returned error: %v", i, err)
		}
		if assigned[name] {
			t.Errorf("Collision detected: name %q assigned twice", name)
		}
		assigned[name] = true
	}
}

func TestFunnyNameGenerator_AvailabilityTracking(t *testing.T) {
	gen := NewFunnyNameGenerator(77)

	invalidName := "NotAValidFunnyName"
	if IsFunnyNameAvailable(invalidName) {
		t.Errorf("IsFunnyNameAvailable(%q) = true for invalid name", invalidName)
	}

	firstName, err := gen.GetRandomName()
	if err != nil {
		t.Fatalf("GetRandomName() returned error: %v", err)
	}

	if !gen.IsUsed(firstName) {
		t.Errorf("After GetRandomName(), IsUsed(%q) = false, expected true", firstName)
	}

	if err := gen.MarkAsAvailable(firstName); err != nil {
		t.Fatalf("MarkAsAvailable(%q) returned error: %v", firstName, err)
	}
	if gen.IsUsed(firstName) {
		t.Errorf("After MarkAsAvailable(%q), IsUsed(%q) = true, expected false", firstName, firstName)
	}
}

func BenchmarkValidateFunnyNameFormat(b *testing.B) {
	for i := 0; i < b.N; i++ {
		ValidateFunnyNameFormat("Mickey")
	}
}

func BenchmarkIsValidFunnyName(b *testing.B) {
	for i := 0; i < b.N; i++ {
		IsValidFunnyName("Mickey")
	}
}

func BenchmarkFunnyNameGenerator_GetRandomName(b *testing.B) {
	gen := NewFunnyNameGenerator(42)
	for i := 0; i < b.N; i++ {
		if i%100 == 0 {
			gen.Reset()
		}
		if _, err := gen.GetRandomName(); err != nil {
			b.Fatalf("GetRandomName() iteration %d returned error: %v", i, err)
		}
	}
}
