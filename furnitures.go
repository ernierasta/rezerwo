package main

import (
	"fmt"
	"sort"
)

func TableNr(ff []Furniture) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "table" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

func ChairNr(ff []Furniture) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "chair" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

func ObjectNr(ff []Furniture) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "object" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

func LabelNr(ff []Furniture) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "label" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

// FurnitureRenumber works only for filtered Furnitures, f.e. only
// chairs for given room
func FurnitureRenumber(ff []Furniture) ([]Furniture, error) {
	roomID := int64(-2)
	ftype := "init"

	// sort numbers, than assign numbers from 1:len(ff) to it
	sort.Slice(ff, func(i, j int) bool {
		return ff[i].Number < ff[j].Number
	})
	for i := range ff {
		ff[i].Number = int64(i + 1)
		err := protectFurnitureTypeRoom(ftype, ff[i].Type, roomID, ff[i].RoomID)
		if err != nil {
			return ff, err
		}

	}
	return ff, nil
}

// FurnitureFull

func TableNrFull(ff []FurnitureFull) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "table" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

func ChairNrFull(ff []FurnitureFull) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "chair" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

func ObjectNrFull(ff []FurnitureFull) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "object" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

func LabelNrFull(ff []FurnitureFull) int64 {
	i := int64(0)
	for _, f := range ff {
		if f.Type == "label" && f.Number > i {
			i = f.Number
		}
	}
	return i
}

// FurnitureRenumber works only for filtered Furnitures, f.e. only
// chairs for given room
func FurnitureRenumberFull(ff []Furniture) ([]Furniture, error) {
	roomID := int64(-2)
	ftype := "init"

	// sort numbers, than assign numbers from 1:len(ff) to it
	sort.Slice(ff, func(i, j int) bool {
		return ff[i].Number < ff[j].Number
	})
	for i := range ff {
		ff[i].Number = int64(i + 1)
		err := protectFurnitureTypeRoom(ftype, ff[i].Type, roomID, ff[i].RoomID)
		if err != nil {
			return ff, err
		}
	}
	return ff, nil
}

func protectFurnitureTypeRoom(newType, oldType string, newRoom, oldRoom int64) error {
	if oldRoom != newRoom && oldRoom != -2 {
		return fmt.Errorf("FurnitureRenumberFull: mixed rooms given, previous: %d, current: %d",
			oldRoom, newRoom)
	}
	oldRoom = newRoom
	if oldType != newType && oldType != "init" {
		return fmt.Errorf("FurnitureRenumberFull: mixed furniture types given, previous: %s, current: %s",
			oldType, newType)
	}
	oldType = newType
	return nil
}
