package main

import "sort"

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

// FurnitureRenumber works only for filtered Furnitures, only
// chairs f.e.
func FurnitureRenumber(ff []Furniture) []Furniture {
	//TODO: sort objects, than assign numbers from 1:len(ff) to it
	sort.Slice(ff, func(i, j int) bool {
		return ff[i].Number < ff[j].Number
	})
	for i := range ff {
		ff[i].Number = int64(i + 1)
	}
	return ff
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

// FurnitureRenumber works only for filtered Furnitures, only
// chairs f.e.
func FurnitureRenumberFull(ff []Furniture) []Furniture {
	//TODO: sort objects, than assign numbers from 1:len(ff) to it
	sort.Slice(ff, func(i, j int) bool {
		return ff[i].Number < ff[j].Number
	})
	for i := range ff {
		ff[i].Number = int64(i + 1)
	}
	return ff
}
