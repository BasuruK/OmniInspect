package domain

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
)

// ==========================================
// Constants
// ==========================================

const (
	MinFunnyNameLength = 3
	MaxFunnyNameLength = 30
)

// ==========================================
// Funny Name Value Object
// ==========================================

type FunnyName struct {
	name string
}

// NewFunnyName creates a validated funny-name value object.
func NewFunnyName(name string) (FunnyName, error) {
	if err := ValidateFunnyNameFormat(name); err != nil {
		return FunnyName{}, fmt.Errorf("NewFunnyName: %w", err)
	}
	if !isFunnyNameInList(name) {
		return FunnyName{}, fmt.Errorf("NewFunnyName: %w", ErrInvalidFunnyName)
	}
	return FunnyName{name: name}, nil
}

// ==========================================
// Getters (Read-Only Accessors)
// ==========================================

// Name returns the funny name value.
func (f FunnyName) Name() string {
	return f.name
}

// String returns the funny name as a string.
func (f FunnyName) String() string {
	return f.Name()
}

// IsValid returns true if the FunnyName is valid (non-empty and passes funny-name validation).
func (f FunnyName) IsValid() bool {
	return IsValidFunnyName(f.name)
}

// ==========================================
// Generator Support Types
// ==========================================

type funnyNameEntry struct {
	funnyName FunnyName
	used      bool
}

type FunnyNameGenerator struct {
	mu     sync.Mutex
	names  []funnyNameEntry
	used   map[string]bool
	source *rand.Rand
}

// ==========================================
// Generator State
// ==========================================

var (
	defaultGenerator *FunnyNameGenerator
	once             sync.Once
)

// ==========================================
// Generator Construction
// ==========================================

func newFunnyNameGenerator(seed int64) *FunnyNameGenerator {
	names := make([]funnyNameEntry, len(funnyNameList))
	for i, name := range funnyNameList {
		funnyName, err := NewFunnyName(strings.ToUpper(name))
		if err != nil {
			panic(fmt.Sprintf("initialize funny name generator: %v", err))
		}
		names[i] = funnyNameEntry{funnyName: funnyName}
	}
	src := rand.NewSource(seed)
	return &FunnyNameGenerator{
		names:  names,
		used:   make(map[string]bool),
		source: rand.New(src),
	}
}

// DefaultFunnyNameGenerator returns the singleton FunnyNameGenerator instance.
// The generator is initialized once with a deterministic seed for reproducibility.
func DefaultFunnyNameGenerator() *FunnyNameGenerator {
	once.Do(func() {
		defaultGenerator = newFunnyNameGenerator(42)
	})
	return defaultGenerator
}

// NewFunnyNameGenerator creates a new FunnyNameGenerator with a given random seed.
// The generator maintains state of used names and provides thread-safe name assignment.
func NewFunnyNameGenerator(seed int64) *FunnyNameGenerator {
	return newFunnyNameGenerator(seed)
}

// ==========================================
// Generator Operations
// ==========================================

// AvailableCount returns the number of funny names that are still available (not yet assigned).
func (g *FunnyNameGenerator) AvailableCount() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	count := 0
	for _, fn := range g.names {
		if !fn.used {
			count++
		}
	}
	return count
}

// IsUsed reports whether the given funny name is currently marked as used.
func (g *FunnyNameGenerator) IsUsed(name string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	return g.used[strings.ToUpper(trimmed)]
}

// MarkAsUsed marks the given funny name as used so it won't be returned by GetRandomName.
// Returns an error if the name is empty, whitespace-only, or not found in the curated list.
func (g *FunnyNameGenerator) MarkAsUsed(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("mark funny name as used: %w", ErrInvalidFunnyName)
	}
	upper := strings.ToUpper(trimmed)
	for i := range g.names {
		if g.names[i].funnyName.Name() == upper {
			g.names[i].used = true
			g.used[upper] = true
			return nil
		}
	}
	return fmt.Errorf("mark funny name as used: %w", ErrInvalidFunnyName)
}

// MarkAsAvailable releases a previously assigned funny name, making it available again.
// Returns an error if the name is empty, whitespace-only, or not found in the curated list.
func (g *FunnyNameGenerator) MarkAsAvailable(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("mark funny name as available: %w", ErrInvalidFunnyName)
	}
	upper := strings.ToUpper(trimmed)
	for i := range g.names {
		if g.names[i].funnyName.Name() == upper {
			g.names[i].used = false
			delete(g.used, upper)
			return nil
		}
	}
	return fmt.Errorf("mark funny name as available: %w", ErrInvalidFunnyName)
}

// GetRandomName returns a randomly selected funny name that is currently available.
// Returns ErrNoAvailableNames when all names have been assigned.
func (g *FunnyNameGenerator) GetRandomName() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	available := make([]int, 0)
	for i, fn := range g.names {
		if !fn.used {
			available = append(available, i)
		}
	}

	if len(available) == 0 {
		return "", ErrNoAvailableNames
	}

	idx := available[g.source.Intn(len(available))]
	g.names[idx].used = true
	name := g.names[idx].funnyName.Name()
	g.used[strings.ToUpper(name)] = true

	return name, nil
}

// Reset marks all funny names as available and clears the used set.
func (g *FunnyNameGenerator) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for i := range g.names {
		g.names[i].used = false
	}
	g.used = make(map[string]bool)
}

// ==========================================
// Validation Helpers
// ==========================================

// ValidateFunnyNameFormat validates that a name conforms to the funny name format.
// Valid names must be 3-30 characters, contain only letters and underscores, and not be empty.
func ValidateFunnyNameFormat(name string) error {
	if name == "" {
		return ErrInvalidFunnyName
	}
	if len(name) < MinFunnyNameLength {
		return ErrFunnyNameTooShort
	}
	if len(name) > MaxFunnyNameLength {
		return ErrFunnyNameTooLong
	}
	for _, c := range name {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || c == '_') {
			return fmt.Errorf("%w: %q contains invalid characters (only letters and underscores allowed)", ErrInvalidFunnyName, name)
		}
	}
	return nil
}

// IsValidFunnyName reports whether the given name is a valid funny name from the curated list.
// Validation checks format (letters and underscores only, 3-30 chars) and list membership.
// Comparison is case-insensitive.
func IsValidFunnyName(name string) bool {
	if err := ValidateFunnyNameFormat(name); err != nil {
		return false
	}
	return isFunnyNameInList(name)
}

func isFunnyNameInList(name string) bool {
	upper := strings.ToUpper(name)
	for _, valid := range funnyNameList {
		if strings.ToUpper(valid) == upper {
			return true
		}
	}
	return false
}

// IsFunnyNameAvailable reports whether a funny name is both valid and not currently assigned.
func IsFunnyNameAvailable(name string) bool {
	if !IsValidFunnyName(name) {
		return false
	}
	return !DefaultFunnyNameGenerator().IsUsed(name)
}

// ==========================================
// Curated Funny Name List
// ==========================================

var funnyNameList = []string{
	"ABBY",
	"ARCHIE",
	"ASTRO",
	"AZrael",
	"Barnacle",
	"Biscuit",
	"Boomer",
	"Bubbles",
	"Bugs",
	"Buster",
	"Buzz",
	"Caillou",
	"Cartoon",
	"Casper",
	"Chester",
	"Chewie",
	"Clifford",
	"Coco",
	"Courage",
	"Cow",
	"Daffy",
	"Daisy",
	"Dexter",
	"Dino",
	"Dobbie",
	"Donald",
	"Dottie",
	"Douglas",
	"Drake",
	"Duke",
	"Dylan",
	"Eeyore",
	"Elmo",
	"Felix",
	"Figaro",
	"Fiona",
	"Fireball",
	"Flounder",
	"Foot",
	"Frankie",
	"Garfield",
	"George",
	"Giggles",
	"Goofy",
	"Grandpa",
	"Griffin",
	"Hamlet",
	"Hank",
	"Hercules",
	"Hobbes",
	"Hoo",
	"Hunter",
	"Igor",
	"Ivy",
	"Jack",
	"Jafar",
	"Jasmine",
	"Jellybean",
	"Jerry",
	"Kaa",
	"Kermit",
	"Kiki",
	"Larry",
	"Lassie",
	"Leo",
	"Lilo",
	"Lola",
	"Lucky",
	"Luna",
	"Maddie",
	"Marvin",
	"Max",
	"Maya",
	"Mickey",
	"Milo",
	"Minnie",
	"Moe",
	"Morty",
	"Muffin",
	"Nala",
	"Nemo",
	"Nibbles",
	"Odie",
	"Olaf",
	"Oliver",
	"Ollie",
	"Oscar",
	"Panda",
	"Peanut",
	"Pebbles",
	"Pete",
	"Pickles",
	"Porky",
	"Quacker",
	"Quentin",
	"Rafiki",
	"Ralph",
	"Rex",
	"Roadrunner",
	"Rover",
	"Ruby",
	"Salem",
	"Sam",
	"Sandy",
	"Scooby",
	"Scout",
	"Sebastian",
	"Shrek",
	"Simba",
	"Skimbles",
	"Smokey",
	"Sniffles",
	"Snoopy",
	"Sofia",
	"Sonic",
	"Sparky",
	"Spencer",
	"Spirit",
	"Spongebob",
	"Spot",
	"Stanley",
	"Stuart",
	"Styx",
	"Sugar",
	"Sunny",
	"Taz",
	"Thomas",
	"Tiger",
	"Tigger",
	"Tito",
	"Tommy",
	"Tony",
	"Tweety",
	"Tom",
	"Tycho",
	"Vanilla",
	"Vinnie",
	"Waffles",
	"Widget",
	"Wiley",
	"Winston",
	"Woody",
	"Zazu",
	"Ziggy",
	"Zippy",
	"Zorro",
}
