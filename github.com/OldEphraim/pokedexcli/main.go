package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/OldEphraim/pokedexcli/internal/pokecache"
)

type Config struct {
	Next      *string        `json:"next"`
	Previous  *string        `json:"previous"`
	Locations []LocationArea `json:"results"`
}

type LocationArea struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type PokemonEncounter struct {
	Pokemon        NamedAPIResource         `json:"pokemon"`         // The Pokémon being encountered
	VersionDetails []VersionEncounterDetail `json:"version_details"` // List of version details
}

type NamedAPIResource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type VersionEncounterDetail struct {
	Version          NamedAPIResource  `json:"version"`
	EncounterDetails []EncounterDetail `json:"encounter_details"`
}

type EncounterDetail struct {
	MinLevel int `json:"min_level"`
	MaxLevel int `json:"max_level"`
	// Add other fields as needed
}

type PokemonDetail struct {
	Name           string     `json:"name"`
	BaseExperience int        `json:"base_experience"`
	Height         int        `json:"height"`
	Weight         int        `json:"weight"`
	Stats          []Stat     `json:"stats"`
	Types          []TypeInfo `json:"types"`
}

type Stat struct {
	Stat     StatInfo `json:"stat"`
	BaseStat int      `json:"base_stat"`
}

type StatInfo struct {
	Name string `json:"name"`
}

type TypeInfo struct {
	Type NamedAPIResource `json:"type"`
}

// User's Pokedex to store caught Pokémon
var userPokedex = make(map[string]PokemonDetail)

// Define the structure for command handlers
type cliCommand struct {
	name        string
	description string
	callback    func(*Config, []string) error
}

// Create a new random generator
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

func main() {
	// Create a config instance to hold pagination data
	config := &Config{}

	// Create a cache instance
	cache := pokecache.NewCache(time.Minute * 5) // Example cache interval of 5 minutes

	// Define the command map
	commands := map[string]cliCommand{
		"help": {
			name:        "help",
			description: "Displays a help message",
			callback:    commandHelp,
		},
		"exit": {
			name:        "exit",
			description: "Exit the Pokedex",
			callback:    commandExit,
		},
		"map": {
			name:        "map",
			description: "Displays the next 20 location areas",
			callback: func(config *Config, args []string) error {
				return commandMap(config, cache) // Pass cache to commandMap
			},
		},
		"mapb": {
			name:        "mapb",
			description: "Displays the previous 20 location areas",
			callback: func(config *Config, args []string) error {
				return commandMapBack(config, cache) // Pass cache to commandMapBack
			},
		},
		"explore": {
			name:        "explore",
			description: "Explore a specific location area and list all Pokémon found there",
			callback: func(config *Config, args []string) error {
				// Expect the first argument to be the location area name
				if len(args) < 1 {
					return fmt.Errorf("please provide a location area name to explore")
				}
				locationName := args[0]
				return commandExplore(config, cache, locationName)
			},
		},
		"catch": {
			name:        "catch",
			description: "Catch a Pokémon by name",
			callback: func(config *Config, args []string) error {
				if len(args) < 1 {
					return fmt.Errorf("please provide a Pokémon name to catch")
				}
				pokemonName := args[0]
				return commandCatch(pokemonName)
			},
		},
		"inspect": {
			name:        "inspect",
			description: "Inspect a caught Pokémon",
			callback: func(config *Config, args []string) error {
				if len(args) < 1 {
					return fmt.Errorf("please provide a Pokémon name to inspect")
				}
				pokemonName := args[0]
				return commandInspect(pokemonName)
			},
		},
		"pokedex": {
			name:        "pokedex",
			description: "List all caught Pokémon",
			callback: func(config *Config, args []string) error {
				return commandPokedex()
			},
		},
	}

	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Welcome to the Pokedex!")
	fmt.Print("Pokedex > ")

	for {
		// Wait for user input
		scanner.Scan()
		input := scanner.Text()

		inputParts := strings.Fields(input) // Split input into command and arguments

		if len(inputParts) == 0 {
			fmt.Println("Unknown command. Type 'help' for a list of commands.")
			continue
		}

		commandName := inputParts[0]
		args := inputParts[1:] // Any additional arguments

		// Check if the input command exists in the map
		if cmd, exists := commands[commandName]; exists {
			err := cmd.callback(config, args) // Pass the config to the callback
			if err != nil {
				fmt.Printf("Error: %s\n", err)
			}
		} else {
			fmt.Println("Unknown command. Type 'help' for a list of commands.")
		}

		fmt.Print("Pokedex > ")
	}
}

// Help command
func commandHelp(config *Config, args []string) error {
	// Print the help message
	fmt.Println("Welcome to the Pokedex!")
	fmt.Println("Usage:")
	fmt.Println("help: Displays a help message")
	fmt.Println("exit: Exit the Pokedex")
	fmt.Println("map: Displays the next 20 location areas")
	fmt.Println("mapb: Displays the previous 20 location areas")
	fmt.Println("explore <location_name>: Explore a location area and list all Pokémon found there")
	fmt.Println("catch <pokemon_name>: Try to catch a Pokémon and store it in your Pokedex!")
	fmt.Println("inspect <pokemon_name>: Inspect a Pokémon you have caught and stored in your Pokedex!")
	fmt.Println("pokedex: Lists the Pokémon currently stored in your Pokedex")
	return nil
}

// Exit command
func commandExit(config *Config, args []string) error {
	fmt.Println("Exiting the Pokedex...")
	os.Exit(0)
	return nil
}

func commandMap(config *Config, cache *pokecache.Cache) error {
	var url string
	if len(config.Locations) == 0 {
		// First call, start from the base URL
		url = "https://pokeapi.co/api/v2/location-area/"
	} else if config.Next != nil {
		// Subsequent call, use the next URL
		url = *config.Next
	} else {
		// No more locations to fetch
		return fmt.Errorf("no more location areas to display")
	}

	// Check if data is in the cache
	if cachedData, found := cache.Get(url); found {
		fmt.Println("Using cached data.")
		err := json.Unmarshal(cachedData, config)
		if err != nil {
			return err
		}

		// Print the location names from the cached data
		for _, loc := range config.Locations {
			fmt.Println(loc.Name)
		}
		return nil
	}

	// Fetch the location areas from the API
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching data: %s", response.Status)
	}

	// Read the response body
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Cache the data
	cache.Add(url, data)

	// Unmarshal the data into the config struct
	err = json.Unmarshal(data, config)
	if err != nil {
		return err
	}

	// Print the location names
	for _, loc := range config.Locations {
		fmt.Println(loc.Name)
	}

	return nil
}

func commandMapBack(config *Config, cache *pokecache.Cache) error {
	if config.Previous == nil {
		return fmt.Errorf("no previous locations available")
	}

	url := *config.Previous

	// Check if data is in the cache
	if cachedData, found := cache.Get(url); found {
		fmt.Println("Using cached data.")
		err := json.Unmarshal(cachedData, config)
		if err != nil {
			return err
		}

		// Print the location names from the cached data
		for _, loc := range config.Locations {
			fmt.Println(loc.Name)
		}
		return nil
	}

	// Fetch the previous location areas from the API
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching data: %s", response.Status)
	}

	// Read the response body
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Cache the data
	cache.Add(url, data)

	// Unmarshal the data into the config struct
	err = json.Unmarshal(data, config)
	if err != nil {
		return err
	}

	// Print the location names
	for _, loc := range config.Locations {
		fmt.Println(loc.Name)
	}

	return nil
}

// Explore command
func commandExplore(_ *Config, cache *pokecache.Cache, locationName string) error {
	url := fmt.Sprintf("https://pokeapi.co/api/v2/location-area/%s", locationName)

	// Check if data is in the cache
	if cachedData, found := cache.Get(url); found {
		fmt.Println("Using cached data.")
		return printPokemonFromLocation(cachedData)
	}

	// Fetch the location data from the API
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching data: %s", response.Status)
	}

	// Read the response body
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	// Cache the data
	cache.Add(url, data)

	// Print Pokémon names from the fetched data
	return printPokemonFromLocation(data)
}

// Helper function to parse and print Pokémon names
func printPokemonFromLocation(data []byte) error {
	var locationData struct {
		PokemonEncounters []PokemonEncounter `json:"pokemon_encounters"`
	}

	err := json.Unmarshal(data, &locationData)
	if err != nil {
		return err
	}

	fmt.Println("Found Pokémon:")
	for _, encounter := range locationData.PokemonEncounters {
		fmt.Printf(" - %s\n", encounter.Pokemon.Name)
	}

	return nil
}

// Catch command
func commandCatch(pokemonName string) error {
	url := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%s", pokemonName)

	// Fetch Pokémon details from the API
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching data: %s", response.Status)
	}

	var pokemonDetail PokemonDetail
	err = json.NewDecoder(response.Body).Decode(&pokemonDetail)
	if err != nil {
		return err
	}

	// Simulate the catch chance based on base experience
	catchChance := 100 - pokemonDetail.BaseExperience // Calculate catch chance (higher experience = lower chance)
	randomValue := rng.Intn(100)                      // Random number between 0 and 99

	fmt.Printf("Throwing a Pokeball at %s...\n", pokemonDetail.Name)
	if randomValue < catchChance {
		fmt.Printf("%s was caught!\n", pokemonDetail.Name)
		userPokedex[pokemonDetail.Name] = pokemonDetail // Store the caught Pokémon
	} else {
		fmt.Printf("%s escaped!\n", pokemonDetail.Name)
	}

	return nil
}

// Inspect command
func commandInspect(pokemonName string) error {
	// Check if the Pokémon has been caught
	pokemon, caught := userPokedex[pokemonName]
	if !caught {
		fmt.Println("you have not caught that pokemon")
		return nil
	}

	// Print Pokémon details
	fmt.Printf("Name: %s\n", pokemon.Name)
	fmt.Printf("Height: %d\n", pokemon.Height)
	fmt.Printf("Weight: %d\n", pokemon.Weight)

	// Print stats
	fmt.Println("Stats:")
	for _, stat := range pokemon.Stats {
		fmt.Printf("  -%s: %d\n", stat.Stat.Name, stat.BaseStat)
	}

	// Print types
	fmt.Println("Types:")
	for _, pokemonType := range pokemon.Types {
		fmt.Printf("  - %s\n", pokemonType.Type.Name)
	}

	return nil
}

// Pokedex command to list caught Pokémon
func commandPokedex() error {
	if len(userPokedex) == 0 {
		fmt.Println("Your Pokedex is empty.")
		return nil
	}

	fmt.Println("Your Pokedex:")
	for name := range userPokedex {
		fmt.Printf(" - %s\n", name)
	}
	return nil
}
