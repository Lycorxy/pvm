//go:build ignore
// +build ignore

// gen-syso generates a Windows COFF .syso file with application manifest embedded.
// This is a pure Go implementation that doesn't require any external tools.
//
// The manifest tells Windows:
//   - This is a normal user-level app (asInvoker), not an admin-elevated one
//   - It's compatible with Windows 7/8/8.1/10/11
//   - It's DPI-aware
//
// This significantly reduces antivirus false positives because:
//  1. The exe now has a proper Windows manifest (no manifest = suspicious)
//  2. asInvoker shows we don't request admin rights
//  3. Compatibility declarations make Windows treat us as a known app
//
// Run: go run scripts/gen-syso.go
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"
)

func main() {
	manifest, err := os.ReadFile("pvm.manifest")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: read manifest: %v\n", err)
		os.Exit(1)
	}

	archs := []struct {
		name    string
		machine uint16
	}{
		{"amd64", 0x8664}, // IMAGE_FILE_MACHINE_AMD64
		{"386", 0x14c},    // IMAGE_FILE_MACHINE_I386
		{"arm64", 0xaa64}, // IMAGE_FILE_MACHINE_ARM64
	}

	for _, arch := range archs {
		outName := "rsrc_windows_" + arch.name + ".syso"
		if err := generateSYSO(outName, arch.machine, manifest); err != nil {
			fmt.Fprintf(os.Stderr, "WARNING: %s: %v\n", arch.name, err)
			continue
		}
		fmt.Printf("  -> %s\n", outName)
	}

	fmt.Println("Done. 'go build' will auto-link these .syso files on Windows.")
}

func generateSYSO(filename string, machine uint16, manifest []byte) error {
	// COFF Object File Format for Windows resources
	// Reference: https://docs.microsoft.com/en-us/windows/win32/debug/pe-format

	// We need to create a COFF object with a .rsrc section containing
	// the manifest as resource type RT_MANIFEST (24), ID 1

	// Build the resource data first
	manifestData := align4(manifest)

	// Resource directory structure:
	// Root -> Type (RT_MANIFEST=24) -> ID (1) -> Data Entry -> Data
	typeOffset := uint32(0)
	idOffset := typeOffset + 16        // one dir entry
	dataEntryOffset := idOffset + 16   // one dir entry
	dataOffset := dataEntryOffset + 16 // one data entry

	// Build .rsrc section content
	var rsrc []byte

	// Root Directory (IMAGE_RESOURCE_DIRECTORY)
	rsrc = append(rsrc, le32(0)...) // Characteristics
	rsrc = append(rsrc, le32(0)...) // TimeDateStamp
	rsrc = append(rsrc, le16(0)...) // MajorVersion
	rsrc = append(rsrc, le16(0)...) // MinorVersion
	rsrc = append(rsrc, le16(0)...) // NumberOfNamedEntries
	rsrc = append(rsrc, le16(1)...) // NumberOfIdEntries
	// Entry: Type = RT_MANIFEST (24), points to idOffset
	rsrc = append(rsrc, le32(24)...)                  // Name/Type ID
	rsrc = append(rsrc, le32(idOffset|0x80000000)...) // OffsetToDirectory (high bit = directory)

	// Type Directory (IMAGE_RESOURCE_DIRECTORY)
	rsrc = append(rsrc, le32(0)...) // Characteristics
	rsrc = append(rsrc, le32(0)...) // TimeDateStamp
	rsrc = append(rsrc, le16(0)...) // MajorVersion
	rsrc = append(rsrc, le16(0)...) // MinorVersion
	rsrc = append(rsrc, le16(0)...) // NumberOfNamedEntries
	rsrc = append(rsrc, le16(1)...) // NumberOfIdEntries
	// Entry: ID = 1, points to dataEntryOffset
	rsrc = append(rsrc, le32(1)...)                          // Name/ID
	rsrc = append(rsrc, le32(dataEntryOffset|0x80000000)...) // OffsetToDirectory

	// ID Directory (IMAGE_RESOURCE_DIRECTORY)
	rsrc = append(rsrc, le32(0)...) // Characteristics
	rsrc = append(rsrc, le32(0)...) // TimeDateStamp
	rsrc = append(rsrc, le16(0)...) // MajorVersion
	rsrc = append(rsrc, le16(0)...) // MinorVersion
	rsrc = append(rsrc, le16(0)...) // NumberOfNamedEntries
	rsrc = append(rsrc, le16(1)...) // NumberOfIdEntries
	// Entry: Language = 0, points to data entry
	rsrc = append(rsrc, le32(0)...)               // Name/Language ID
	rsrc = append(rsrc, le32(dataEntryOffset)...) // OffsetToData (no high bit = data entry)

	// Data Entry (IMAGE_RESOURCE_DATA_ENTRY)
	rsrc = append(rsrc, le32(dataOffset)...)            // OffsetToData (from section start)
	rsrc = append(rsrc, le32(uint32(len(manifest)))...) // Size
	rsrc = append(rsrc, le32(0x040904b0)...)            // CodePage (en-US, Unicode)
	rsrc = append(rsrc, le32(0)...)                     // Reserved

	// Manifest data
	rsrc = append(rsrc, manifestData...)

	// Now build the COFF object
	// Section alignment
	sectionAlign := uint32(1)
	fileAlign := uint32(1)

	// COFF Header
	coffHeader := []byte{}
	coffHeader = append(coffHeader, le16(machine)...) // Machine
	coffHeader = append(coffHeader, le16(1)...)       // NumberOfSections
	coffHeader = append(coffHeader, le32(0)...)       // TimeDateStamp
	coffHeader = append(coffHeader, le32(0)...)       // PointerToSymbolTable
	coffHeader = append(coffHeader, le16(1)...)       // NumberOfSymbols
	coffHeader = append(coffHeader, le16(0)...)       // SizeOfOptionalHeader
	coffHeader = append(coffHeader, le16(0)...)       // Characteristics

	// Section Header (.rsrc)
	sectionHeader := []byte{}
	name := ".rsrc"
	nameBytes := make([]byte, 8)
	copy(nameBytes, name)
	sectionHeader = append(sectionHeader, nameBytes...)                        // Name
	sectionHeader = append(sectionHeader, le32(uint32(len(rsrc)))...)          // VirtualSize
	sectionHeader = append(sectionHeader, le32(0)...)                          // VirtualAddress
	sectionHeader = append(sectionHeader, le32(uint32(len(rsrc)))...)          // SizeOfRawData
	sectionHeader = append(sectionHeader, le32(uint32(len(coffHeader)+40))...) // PointerToRawData
	sectionHeader = append(sectionHeader, le32(0)...)                          // PointerToRelocations
	sectionHeader = append(sectionHeader, le32(0)...)                          // PointerToLinenumbers
	sectionHeader = append(sectionHeader, le16(0)...)                          // NumberOfRelocations
	sectionHeader = append(sectionHeader, le16(0)...)                          // NumberOfLinenumbers
	sectionHeader = append(sectionHeader, le32(0x40000040)...) // Characteristics (CNT_INITIALIZED_DATA|MEM_READ)

	_ = sectionAlign
	_ = fileAlign

	// String table (4 bytes: size including the size field itself)
	stringTable := le32(4)

	// Symbol table entry for .rsrc section
	symbol := []byte{}
	symName := make([]byte, 8)
	copy(symName, ".rsrc")
	symbol = append(symbol, symName...) // Short name
	symbol = append(symbol, le32(0)...) // Value
	symbol = append(symbol, le16(1)...) // SectionNumber
	symbol = append(symbol, le16(0)...) // Type (NULL)
	symbol = append(symbol, byte(3))    // StorageClass (IMAGE_SYM_CLASS_STATIC)
	symbol = append(symbol, byte(0))    // NumberOfAuxSymbols

	// Assemble the COFF file
	var coff []byte
	coff = append(coff, coffHeader...)
	coff = append(coff, sectionHeader...)
	coff = append(coff, rsrc...)
	coff = append(coff, symbol...)
	coff = append(coff, stringTable...)

	return os.WriteFile(filename, coff, 0644)
}

func le16(v uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, v)
	return b
}

func le32(v uint32) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	return b
}

func align4(data []byte) []byte {
	rem := len(data) % 4
	if rem == 0 {
		return data
	}
	return append(data, make([]byte, 4-rem)...)
}

// Suppress unused import warning
var _ = strings.NewReader
