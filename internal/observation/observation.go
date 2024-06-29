package observation

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/tphakala/birdnet-go/internal/conf"
	"github.com/tphakala/birdnet-go/internal/datastore"
)

// ParseSpeciesString extracts the scientific name, common name, and species code from the species string.
func ParseSpeciesString(species string) (string, string, string) {
	parts := strings.SplitN(species, "_", 3) // Split into 3 parts at most: scientificName, commonName, speciesCode
	if len(parts) == 3 {
		// Return scientificName (parts[0]), commonName (parts[1]), and speciesCode (parts[2])
		return parts[0], parts[1], parts[2]
	}
	// Log this to see what is being returned
	fmt.Printf("Species string has an unexpected format: %s\n", species)
	// Return the original species string for all parts if the format doesn't match the expected
	return species, species, ""
}

// New creates and returns a new Note with the provided parameters and current date and time.
// It uses the configuration and parsing functions to set the appropriate fields.
func New(settings *conf.Settings, beginTime, endTime float64, species string, confidence float64, source string, clipName string, elapsedTime time.Duration) datastore.Note {
	// Parse the species string to get the scientific name, common name, and species code.
	scientificName, commonName, speciesCode := ParseSpeciesString(species)

	// detectionTime is time now minus 3 seconds to account for the delay in the detection
	now := time.Now()
	date := now.Format("2006-01-02")
	detectionTime := now.Add(-2 * time.Second)
	time := detectionTime.Format("15:04:05")

	var audioSource string
	if settings.Input.Path != "" {
		audioSource = settings.Input.Path
	} else {
		audioSource = source
	}

	// Return a new Note struct populated with the provided parameters as well as the current date and time.
	return datastore.Note{
		SourceNode: settings.Main.Name, // From the provided configuration settings.
		Date:       date,               // Use ISO 8601 date format.
		Time:       time,               // Use 24-hour time format.
		Source:     audioSource,        // From the provided configuration settings.
		//BeginTime:      beginTime,                    // Start time of the observation.
		//EndTime:        endTime,                      // End time of the observation.
		SpeciesCode:    speciesCode,                  // Parsed species code.
		ScientificName: scientificName,               // Parsed scientific name of the species.
		CommonName:     commonName,                   // Parsed common name of the species.
		Confidence:     confidence,                   // Confidence score of the observation.
		Latitude:       settings.BirdNET.Latitude,    // Geographic latitude where the observation was made.
		Longitude:      settings.BirdNET.Longitude,   // Geographic longitude where the observation was made.
		Threshold:      settings.BirdNET.Threshold,   // Threshold setting from configuration.
		Sensitivity:    settings.BirdNET.Sensitivity, // Sensitivity setting from configuration.
		ClipName:       clipName,                     // Name of the audio clip.
		ProcessingTime: elapsedTime,                  // Time taken to process the observation.
	}
}

// WriteNotesTable writes a slice of Note structs to a table-formatted text output.
// The output can be directed to either stdout or a file specified by the filename.
// If the filename is an empty string, it writes to stdout.
func WriteNotesTable(settings *conf.Settings, notes []datastore.Note, filename string) error {
	var w io.Writer
	// Determine the output destination based on the filename argument.
	if filename == "" {
		w = os.Stdout
	} else {
		// Ensure the filename has a .txt extension.
		if !strings.HasSuffix(filename, ".txt") {
			filename += ".txt"
		}
		// Create or truncate the file with the specified filename.
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file: %v", err)
		}
		defer file.Close() // Ensure the file is closed when the function exits.
		w = file
	}

	// Write the header to the output destination.
	header := "Selection\tView\tChannel\tBegin File\tBegin Time (s)\tEnd Time (s)\tLow Freq (Hz)\tHigh Freq (Hz)\tSpecies Code\tCommon Name\tConfidence\n"
	if _, err := w.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header: %v", err)
	}

	// Pre-declare err outside the loop to avoid re-declaration
	var err error

	for i, note := range notes {
		if note.Confidence <= settings.BirdNET.Threshold {
			continue // Skip the current iteration as the note doesn't meet the threshold
		}

		// Prepare the line for notes above the threshold, assuming note.BeginTime and note.EndTime are of type time.Time
		line := fmt.Sprintf("%d\tSpectrogram 1\t1\t%s\t%s\t%s\t0\t15000\t%s\t%s\t%.4f\n",
			i+1, note.Source, note.BeginTime.Format("15:04:05"), note.EndTime.Format("15:04:05"), note.SpeciesCode, note.CommonName, note.Confidence)

		// Attempt to write the note
		if _, err = w.Write([]byte(line)); err != nil {
			break // If an error occurs, exit the loop
		}
	}

	// Check if an error occurred during the loop and return it
	if err != nil {
		return fmt.Errorf("failed to write note: %v", err)
	} else if filename != "" {
		fmt.Println("Output written to", filename)
	}

	// Return nil if the writing operation completes successfully.
	return nil
}

// WriteNotesCsv writes the slice of notes to the specified destination in CSV format.
// If filename is an empty string, the function writes to stdout.
// The function returns an error if writing to the destination fails.
func WriteNotesCsv(settings *conf.Settings, notes []datastore.Note, filename string) error {
	// Define an io.Writer to abstract the writing operation.
	var w io.Writer

	// Determine the output destination, file or screen
	if settings.Output.File.Enabled {
		// Ensure the filename has a .csv extension.
		if !strings.HasSuffix(filename, ".csv") {
			filename += ".csv"
		}
		// Create or truncate the file with the given filename.
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filename, err)
		}
		defer file.Close()
		w = file
	} else {
		// Print output to stdout if the file output is disabled
		w = os.Stdout
	}

	// Define the CSV header.
	header := "Start (s),End (s),Scientific name,Common name,Confidence\n"
	// Write the header to the output destination.
	if _, err := w.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write header to CSV: %w", err)
	}

	// Pre-declare err outside the loop to avoid re-declaration
	var err error

	for _, note := range notes {
		if note.Confidence <= settings.BirdNET.Threshold {
			continue // Skip the current iteration as the note doesn't meet the threshold
		}

		line := fmt.Sprintf("%s,%s,%s,%s,%.4f\n",
			note.BeginTime.Format("2006-01-02 15:04:05"), // Formats BeginTime
			note.EndTime.Format("2006-01-02 15:04:05"),   // Formats EndTime
			note.ScientificName, note.CommonName, note.Confidence)

		if _, err = w.Write([]byte(line)); err != nil {
			// Break out of the loop at the first sign of an error
			break
		}
	}

	// Handle any errors that occurred during the write operation
	if err != nil {
		return fmt.Errorf("failed to write note to CSV: %w", err)
	} else {
		fmt.Println("Output written to", filename)
	}

	// Return nil if the writing operation completes successfully.
	return nil
}

// WriteNotesJson writes the slice of notes to the specified destination in JSON format.
// If filename is an empty string, the function writes to stdout.
// The function returns an error if writing to the destination fails.
func WriteNotesJson(settings *conf.Settings, notes []datastore.Note, filename string) error {
	// Define an io.Writer to abstract the writing operation.
	var w io.Writer

	// Determine the output destination, file or screen
	if settings.Output.File.Enabled {
		// Ensure the filename has a .csv extension.
		if !strings.HasSuffix(filename, ".json") {
			filename += ".json"
		}
		// Create or truncate the file with the given filename.
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filename, err)
		}
		defer file.Close()
		w = file
	} else {
		// Print output to stdout if the file output is disabled
		w = os.Stdout
	}

	// Pre-declare err outside the loop to avoid re-declaration
	var err error

	noteJson, err := json.Marshal(notes)
	if err != nil {
		return fmt.Errorf("failed to convert notes to JSON: %w", err)
	}

	if _, err = w.Write(noteJson); err != nil {
		return fmt.Errorf("couldn't save note in JSON format to %s: %w", filename, err)
	}

	// Return nil if the writing operation completes successfully.
	return nil
}
