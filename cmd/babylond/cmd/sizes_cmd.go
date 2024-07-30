package cmd

import (
	"cmp"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"
)

const (
	FlagPrintInterval = "print-interval"
)

// r is a regular expression that matched the store key prefix
// we cannot use modules names direclty as sometimes module key != store key
// for example account module has store key "acc" and module key "auth"
var r, _ = regexp.Compile("s/k:[A-Za-z]+/")

func OpenDB(dir string) (dbm.DB, error) {
	fmt.Printf("Opening database at: %s\n", dir)
	defer fmt.Printf("Opened database at: %s\n", dir)

	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(dir)
	if !strings.EqualFold(ext, ".db") {
		return nil, fmt.Errorf("database directory must end with .db")
	}

	directory := filepath.Dir(dir)
	filename := filepath.Base(dir)

	name := strings.TrimSuffix(filename, ext)
	db, err := dbm.NewGoLevelDB(name, directory, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

type ModuleStats struct {
	ModuleKey      string
	NodeCount      uint64
	TotalSizeBytes uint64
}

type GlobalStats struct {
	TotalNodeCount       uint64
	TotalSizeBytes       uint64
	UnknownStoreKeyCount uint64
	UnknownStoreKeySize  uint64
}

func extractStoreKey(fullKey string) string {
	return r.FindString(fullKey)
}

func printModuleStats(stats map[string]*ModuleStats, gs *GlobalStats) {
	if len(stats) == 0 {
		fmt.Printf("No module stats to report\n")
		return
	}

	statsSlice := maps.Values(stats)

	slices.SortStableFunc(statsSlice, func(a *ModuleStats, b *ModuleStats) int {
		// sort from highest to lowest size
		return -cmp.Compare(a.TotalSizeBytes, b.TotalSizeBytes)
	})

	fmt.Printf("****************** Printing module stats ******************\n")
	fmt.Printf("Total number of nodes in db: %d\n", gs.TotalNodeCount)
	fmt.Printf("Total size of database: %d bytes\n", gs.TotalSizeBytes)
	fmt.Printf("Total number of unknown storekeys: %d\n", gs.UnknownStoreKeyCount)
	fmt.Printf("Total size of unknown storekeys: %d bytes\n", gs.UnknownStoreKeySize)
	fmt.Printf("Fraction of unknown storekeys: %.3f\n", float64(gs.UnknownStoreKeySize)/float64(gs.TotalSizeBytes))
	for _, v := range statsSlice {
		fmt.Printf("Store key %s:\n", v.ModuleKey)
		fmt.Printf("Number of tree state nodes: %d\n", v.NodeCount)
		fmt.Printf("Total size of of module storage: %d bytes\n", v.TotalSizeBytes)
		fmt.Printf("Fraction of total size: %.3f\n", float64(v.TotalSizeBytes)/float64(gs.TotalSizeBytes))
	}
	fmt.Printf("****************** Printed stats for all Babylon modules ******************\n")
}

func PrintDBStats(db dbm.DB, printInterval int) {
	fmt.Printf("****************** Starting to iterate over whole database ******************\n")
	storeKeyStats := make(map[string]*ModuleStats)

	gs := GlobalStats{}

	itr, err := db.Iterator(nil, nil)
	if err != nil {
		panic(err)
	}
	fmt.Printf("****************** Retrived database iterator ******************\n")

	defer itr.Close()
	for ; itr.Valid(); itr.Next() {
		gs.TotalNodeCount++
		if gs.TotalNodeCount%uint64(printInterval) == 0 {
			printModuleStats(storeKeyStats, &gs)
		}

		fullKey := itr.Key()
		fullValue := itr.Value()
		fullKeyString := string(fullKey)
		keyValueSize := uint64(len(fullKey) + len(fullValue))
		extractedStoreKey := extractStoreKey(fullKeyString)

		if extractedStoreKey == "" {
			gs.UnknownStoreKeyCount++
			gs.TotalSizeBytes += keyValueSize
			gs.UnknownStoreKeySize += keyValueSize
			continue
		}

		if _, ok := storeKeyStats[extractedStoreKey]; !ok {
			storeKeyStats[extractedStoreKey] = &ModuleStats{
				ModuleKey: extractedStoreKey,
			}
		}

		storeKeyStats[extractedStoreKey].NodeCount++
		storeKeyStats[extractedStoreKey].TotalSizeBytes += keyValueSize
		gs.TotalSizeBytes += keyValueSize
	}

	if err := itr.Error(); err != nil {
		panic(err)
	}
	fmt.Printf("****************** Finished iterating over whole database ******************\n")
	printModuleStats(storeKeyStats, &gs)
}

func ModuleSizeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "module-sizes [path-to-db]",
		Short: "print sizes of each module in the database",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := cmd.Flags().GetInt(FlagPrintInterval)

			if err != nil {
				return err
			}

			if d <= 0 {
				return fmt.Errorf("print interval must be greater than 0")
			}

			pathToDB := args[0]

			db, err := OpenDB(pathToDB)

			if err != nil {
				return err
			}
			defer db.Close()

			PrintDBStats(db, d)

			return nil
		},
	}

	cmd.Flags().Int(FlagPrintInterval, 100000, "interval between printing databse stats")

	return cmd
}
