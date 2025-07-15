package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yumyai/ggtable/logger"
)

// TODO: replace these with DB operation later.
var (
	MAP_HEADER map[string]string = map[string]string{
		// "CBS57885":   "Pythium insidiosum CBS57885 [C1] MGI1",
		// "CBS57985":   "Pythium insidiosum CBS57985 [C1] MGI1",
		// "CBS57785":   "Pythium insidiosum CBS57785 [C1] MGI1",
		// "CBS57585":   "Pythium insidiosum CBS57585 [C1] MGI1",
		// "CBS57385":   "Pythium insidiosum CBS57385 [I]",
		// "CBS57385m":  "Pythium insidiosum CBS57385 [C1] MGI2",
		// "EQ25":       "Pythium insidiosum EQ25 [C1] MGI1",
		// "EQ04":       "Pythium insidiosum EQ04 [C1] MGI1",
		// "CAO":        "Pythium insidiosum CAO [C1] MGI1",
		// "EQ10":       "Pythium insidiosum EQ10 [C1] MGI1",
		// "EQ09":       "Pythium insidiosum EQ09 [C1] MGI1",
		// "EQ05":       "Pythium insidiosum EQ05 [C1] MGI1",
		// "ATCC200269": "Pythium insidiosum ATCC200269 [I]",
		// "P45BR":      "Pythium insidiosum P45BR [C1] MGI1",
		// "CBS101555":  "Pythium insidiosum CBS101555 [I]",
		// "PINS":       "Pythium insidiosum Pi-S [II]",
		// "PIS":        "Pythium insidiosum PIS [C2] MGI2",
		// "PINSPB":     "Pythium insidiosum Pi-S PacBio [II]",
		// "P41NK":      "Pythium insidiosum P41NK [C2] MGI2",
		// "Pi19":       "Pythium insidiosum Pi19 [II]",
		// "P44TW":      "Pythium insidiosum P44TW [C2] MGI1",
		// "46P211CM":   "Pythium insidiosum 46P211CM [C2] MGI2",
		// "P47ZG":      "Pythium insidiosum P47ZG [C2] MGI1",
		// "46P213L8":   "Pythium insidiosum 46P213L8 [C2] MGI2",
		// "RT01":       "Pythium insidiosum RT01 [C2] MGI1",
		// "RT02":       "Pythium insidiosum RT02 [C2] MGI2",
		// "SIMI2989":   "Pythium insidiosum SIMI2989 [C2] MGI2",
		// "SIMI7873":   "Pythium insidiosum SIMI7873 [C2] MGI2",
		// "KCB07":      "Pythium insidiosum KCB07 [C2] MGI2",
		// "CBS101039":  "Pythium insidiosum CBS101039 [C2] MGI1",
		// "P16PC":      "Pythium insidiosum P16PC [C2] MGI1",
		// "KCB02":      "Pythium insidiosum KCB02 [C2] MGI2",
		// "P36SW":      "Pythium insidiosum P36SW [C2] MGI1",
		// "P53LD":      "Pythium insidiosum P53LD [C2] MGI1",
		// "P40KJ":      "Pythium insidiosum P40KJ [C2] MGI2",
		// "CU43150":    "Pythium insidiosum CU43150 [C2] MGI2",
		// "SIMI91646":  "Pythium insidiosum SIMI91646 [C2] MGI2",
		// "M29":        "Pythium insidiosum M29 [C2] MGI2",
		// "P39KP":      "Pythium insidiosum P39KP [C2] MGI2",
		// "MCC18":      "Pythium insidiosum MCC18 [II]",
		// "59P211AT":   "Pythium insidiosum 59P211AT [C2] MGI2",
		// "CR02":       "Pythium insidiosum CR02 [II]",
		// "SIMI452345": "Pythium insidiosum SIMI452345 [C2] MGI2",
		// "46P214L10":  "Pythium insidiosum 46P214L10 [C2] MGI2",
		// "P50PR":      "Pythium insidiosum P50PR [C2] MGI1",
		// "KCB05":      "Pythium insidiosum KCB05 [C2] MGI2",
		// "KAN06":      "Pythium insidiosum KAN06 [II] MGI2",
		// "P42PT":      "Pythium insidiosum P42PT [C2] MGI1",
		// "SIMI8727":   "Pythium insidiosum SIMI8727 [C2] MGI2",
		// "P15ON":      "Pythium insidiosum P15ON [C2] MGI1",
		// "P34UM":      "Pythium insidiosum P34UM [C2] MGI1",
		// "RM902":      "Pythium insidiosum RM902 [C2] MGI2",
		// "MCC5":       "Pythium insidiosum MCC5 [C2] MGI2",
		// "SIMI18093":  "Pythium insidiosum SIMI18093 [C2] MGI2",
		// "P38WA":      "Pythium insidiosum P38WA [C2] MGI1",
		// "ATCC28251":  "Pythium insidiosum ATCC28251 [C2] MGI1",
		// "ATCC64221":  "Pythium insidiosum ATCC64221 [II]",
		// "46P212L4":   "Pythium insidiosum 46P212L4 [C3] MGI2",
		// "Pi049":      "Pythium insidiosum SIMI769548 [C3] MGI1",
		// "P52WN":      "Pythium insidiosum P52WN [C3] MGI1",
		// "P46EP":      "Pythium insidiosum P46EP [C3] MGI1",
		// "P211":       "Pythium insidiosum P211 [C3] MGI1",
		// "P48DZ":      "Pythium insidiosum P48DZ [C3] MGI1",
		// "MCC13":      "Pythium insidiosum MCC13 [III]",
		// "MCC13m":     "Pythium insidiosum MCC13 [C3] MGI2",
		// "MCC17":      "Pythium insidiosum MCC17 [C3] MGI1",
		// "SIMI4763":   "Pythium insidiosum SIMI4763 [III]",
		// "KCB01":      "Pythium insidiosum KCB01 [C3] MGI1",
		// "KCB03":      "Pythium insidiosum KCB03 [C3] MGI1",
		// "KCB08":      "Pythium insidiosum KCB08 [C3] MGI2",
		// "KCB09":      "Pythium insidiosum KCB09 [C3] MGI2",
		// "P43SY":      "Pythium insidiosum P43SY [C3] MGI1",
		// "SIMI330644": "Pythium insidiosum SIMI330644 [C3] MGI1",
		// "SIMI292145": "Pythium insidiosum SIMI292145 [C3] MGI1",
		// "ATCC90586":  "Pythium insidiosum ATCC90586 [C3] MGI1",
		// "PARR":       "Pythium arrhenomanes ATCC 12531",
		// "RM906":      "Pythium catenulatum RM906 MGI1",
		// "RCB01":      "Pythium rhizo-oryzae RCB01 MGI1",
		// "ATCC32230":  "Pythium aphanidermatum ATCC32230 MGI1",
		// "PAPH":       "Pythium aphanidermatum DAOM BR444",
		// "PINF":       "Phytophthora infestans T30-4",
		// "PPAR":       "Phytophthora parasitica INRA-310",
		// "PCAP":       "Phytophthora capsici LT1534",
		// "PRAM":       "Phytophthora ramorum pr102",
		// "PCIN":       "Phytophthora cinnamomi CBS 144.22",
		// "PSOJ":       "Phytophthora sojae P6497",
		// "HARA":       "Hyaloperonospora arabidopsis Emoy2",
		// "PVEX":       "Phytopythium vexans DAOM BR484",
		// "PIRR":       "Pythium irregulare DAOM BR486",
		// "PIWA":       "Pythium iwayamai DAOM BR242034",
		// "PULT":       "Pythium ultimum DAOM BR144",
		// "LGIG":       "Lagenidium giganteum ARSEF373",
		// "SDEC":       "Saprolegnia declina VS20",
		// "SPAR":       "Saprolegnia parasitica CBS_223.65",
		// "AAST":       "Aphanomyces astaci APO3",
		// "AINV":       "Aphanomyces invadans NJM9701",
		// "CBS134681":  "Lagenidium karlingii CBS134681 MGI1",
		// "ACAN":       "Albugo candida 2VRR",
		// "ALAI":       "Albugo laibachii Nc14",
		// "PTRI":       "Phaeodactylum tricornutum CCAP1055-1",
		// "TPSE":       "Thalassiosira pseudonana CCMP1335",
	}

	ALL_GENOME_ID = []string{
		"CBS57885",
		"CBS57985",
		"CBS57785",
		"CBS57585",
		"CBS57385",
		"CBS57385m",
		"EQ25",
		"EQ04",
		"CAO",
		"EQ10",
		"EQ09",
		"EQ05",
		"ATCC200269",
		"P45BR",
		"CBS101555",
		"PINS",
		"PIS",
		"PINSPB",
		"P41NK",
		"Pi19",
		"P44TW",
		"46P211CM",
		"P47ZG",
		"46P213L8",
		"RT01",
		"RT02",
		"SIMI2989",
		"SIMI7873",
		"KCB07",
		"CBS101039",
		"P16PC",
		"KCB02",
		"P36SW",
		"P53LD",
		"P40KJ",
		"CU43150",
		"SIMI91646",
		"M29",
		"P39KP",
		"MCC18",
		"59P211AT",
		"CR02",
		"SIMI452345",
		"46P214L10",
		"P50PR",
		"KCB05",
		"KAN06",
		"P42PT",
		"SIMI8727",
		"P15ON",
		"P34UM",
		"RM902",
		"MCC5",
		"SIMI18093",
		"P38WA",
		"ATCC28251",
		"ATCC64221",
		"46P212L4",
		"Pi049",
		"P52WN",
		"P46EP",
		"P211",
		"P48DZ",
		"MCC13",
		"MCC13m",
		"MCC17",
		"SIMI4763",
		"KCB01",
		"KCB03",
		"KCB08",
		"KCB09",
		"P43SY",
		"SIMI330644",
		"SIMI292145",
		"ATCC90586",
		"PARR",
		"RM906",
		"RCB01",
		"ATCC32230",
		"PAPH",
		"PINF",
		"PPAR",
		"PCAP",
		"PRAM",
		"PCIN",
		"PSOJ",
		"HARA",
		"PVEX",
		"PIRR",
		"PIWA",
		"PULT",
		"LGIG",
		"SDEC",
		"SPAR",
		"AAST",
		"AINV",
		"CBS134681",
		"ACAN",
		"ALAI",
		"PTRI",
		"TPSE",
	}
)

// InitMapHeader loads MAP_HEADER from the DB once during startup.
func InitMapHeader(db *sql.DB) error {
	ctx := context.TODO()

	rows, err := db.QueryContext(ctx, `SELECT genome_id, genome_fullname FROM genome_info`)
	if err != nil {
		return fmt.Errorf("InitMapHeader: query failed: %w", err)
	}
	defer rows.Close()

	m := make(map[string]string)
	for rows.Next() {
		var id, fullname string
		if err := rows.Scan(&id, &fullname); err != nil {
			return fmt.Errorf("InitMapHeader: scan failed: %w", err)
		}
		m[id] = fullname
	}

	MAP_HEADER = m
	return nil
}

func InitGenomeID() error {

	// Check if MAP_HEADER is already initialized
	if len(MAP_HEADER) > 0 {
		// If it is, return the keys directly
		genomeIDs := make([]string, 0, len(MAP_HEADER))
		for id := range MAP_HEADER {
			genomeIDs = append(genomeIDs, id)
		}
		ALL_GENOME_ID = genomeIDs
		return nil
	} else {
		// If not, return an error, says that it is not properly initialized.
		logger.Error("MAP_HEADER is not initialized")
		return fmt.Errorf("MAP_HEADER is not initialized")
	}
}
