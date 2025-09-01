package main

import (
	"encoding/json"
	"os"

	"gopkg.in/yaml.v3"
)

type SchemaEquipmentList struct {
	Equipment []SchemaEquipment `yaml:"equipment_list"`
}

type SchemaEquipment struct {
	Id          string `yaml:"id"`
	Description string `yaml:"description"`
	Category    string `yaml:"category"`
}

type SchemaFundamentalsList struct {
	Fundamentals []SchemaFundamental `yaml:"fundamentals_list"`
}

type SchemaFundamental struct {
	Id          string   `yaml:"id"`
	Description string   `yaml:"description"`
	Examples    []string `yaml:"examples"`
}

func main() {
	b, err := os.ReadFile("schemas/equipment_list.yaml")
	if err != nil {
		panic(err)
	}

	var sel SchemaEquipmentList
	if err := yaml.Unmarshal(b, &sel); err != nil {
		panic(err)
	}

	// Build output YAML structure: equipment_inventory_profiles mapping id -> description
	out := struct {
		EquipmentInventoryProfiles map[string]string `yaml:"equipment_inventory_profiles"`
	}{
		EquipmentInventoryProfiles: make(map[string]string, len(sel.Equipment)),
	}
	for _, e := range sel.Equipment {
		out.EquipmentInventoryProfiles[e.Id] = e.Description
	}

	bOut, err := yaml.Marshal(out)
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("web/equipment.yaml", bOut, 0644); err != nil {
		panic(err)
	}

	// Also update schemas/set-v1.json equipment enum from equipment_list.yaml
	exSchemaPath := "schemas/set-v1.json"
	exb, err := os.ReadFile(exSchemaPath)
	if err != nil {
		panic(err)
	}

	var ex map[string]any
	if err := json.Unmarshal(exb, &ex); err != nil {
		panic(err)
	}

	// Traverse to equipment enum. Support both legacy array schema (items.properties...) and current object schema (properties...)
	var eqItems map[string]any
	if itemsNode, ok := ex["items"].(map[string]any); ok {
		if props, ok := itemsNode["properties"].(map[string]any); ok {
			if eq, ok := props["equipment"].(map[string]any); ok {
				if ei, ok := eq["items"].(map[string]any); ok {
					eqItems = ei
				}
			}
		}
	}
	if eqItems == nil {
		if props, ok := ex["properties"].(map[string]any); ok {
			if eq, ok := props["equipment"].(map[string]any); ok {
				if ei, ok := eq["items"].(map[string]any); ok {
					eqItems = ei
				}
			}
		}
	}
	if eqItems == nil {
		panic("set schema: could not locate properties.equipment.items for enum update")
	}

	// Update fundamentals enum in schemas/set-v1.json from fundamentals_list.yaml
	fb, err := os.ReadFile("schemas/fundamentals_list.yaml")
	if err != nil {
		panic(err)
	}
	var sfl SchemaFundamentalsList
	if err := yaml.Unmarshal(fb, &sfl); err != nil {
		panic(err)
	}
	fundIds := make([]string, 0, len(sfl.Fundamentals))
	for _, f := range sfl.Fundamentals {
		fundIds = append(fundIds, f.Id)
	}

	// Re-read set schema JSON fresh after previous mutation
	exb, err = os.ReadFile(exSchemaPath)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(exb, &ex); err != nil {
		panic(err)
	}
	// Locate properties.pattern.items.enum
	var patternItems map[string]any
	if props, ok := ex["properties"].(map[string]any); ok {
		if pattern, ok := props["pattern"].(map[string]any); ok {
			if pi, ok := pattern["items"].(map[string]any); ok {
				patternItems = pi
			}
		}
	}
	if patternItems == nil {
		panic("set schema: could not locate properties.pattern.items for fundamentals enum update")
	}
	patternItems["enum"] = fundIds

	b2, err := json.MarshalIndent(ex, "", "    ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(exSchemaPath, b2, 0644); err != nil {
		panic(err)
	}

	// Build enum from equipment_list ids
	ids := make([]string, 0, len(sel.Equipment))
	for _, e := range sel.Equipment {
		ids = append(ids, e.Id)
	}
	eqItems["enum"] = ids

	// Marshal updated JSON with indentation
	outJSON, err := json.MarshalIndent(ex, "", "    ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(exSchemaPath, outJSON, 0644); err != nil {
		panic(err)
	}
}
