package businesses

import (
	"restaurants/internal/admin/category"
	"restaurants/internal/admin/item"
	"restaurants/internal/admin/province"
	"restaurants/internal/admin/subcategory"
)

type BusinessesFilter struct {
	SubcategoryId int    `form:"subcategory_id"`
	CategoryId    int    `form:"category_id"`
	ProvinceID    int    `form:"province_id"`
	IsDiscounted  *bool  `form:"is_discounted"`
	OpenTime      string `form:"open_time" time_format:"15:04"`
	ClosesTime    string `form:"closes_time" time_format:"15:04"`
	SortByValue   string `form:"sort_by_value" binding:"omitempty,oneof=ASC DESC"`
	Search        string `form:"search"`
	Limit         int    `form:"limit"`
	Offset        int    `form:"offset"`
	Status        string `form:"status"`
}

type IndexFilter struct {
	CategoryIdRaw     string `form:"category_ids"`
	SubcategoryIdsRaw string `form:"subcategory_ids"`
	ProvinceID        int    `form:"province_id" binding:"required"`
	SubcategoryIds    []int
	CategoryIds       []int
}

type BusinessesReqDTO struct {
	Name           string        `json:"name"`
	ProvinceId     int           `json:"province_id"`
	District       DictionaryDTO `json:"district"`
	Phone          string        `json:"phone"`
	Description    DictionaryDTO `json:"description"`
	CategoryId     *int          `json:"category_id"`
	SubcategoryIds *[]int        `json:"subcategory_ids"`
	OpensTime      string        `json:"opens_time"`
	ClosesTime     string        `json:"closes_time"`
	Value          *float32      `json:"value"`
	Expires        *int          `json:"expires"`
	CanOrder       *bool         `json:"can_order"`
	CanReserve     *bool         `json:"can_reserve"`
}

type BusinessesResDTO struct {
	Id              int                            `json:"id"`
	Name            string                         `json:"name"`
	Address         DictionaryDTO                  `json:"address"`
	Images          []string                       `json:"images"`
	Phone           string                         `json:"phone"`
	Province        province.ProvinceDTO           `json:"province"`
	Description     DictionaryDTO                  `json:"description"`
	Category        *category.CategoryDTO          `json:"category"`
	Subcategory     []subcategory.SubcategoriesDTO `json:"subcategories"`
	Items           []item.ItemGetAllDTO           `json:"items"`
	OpensTime       string                         `json:"opens_time"`
	ClosesTime      string                         `json:"closes_time"`
	DiscountPercent *int                           `json:"discount_percent"`
	Value           *float32                       `json:"value"`
	DiscountValue   *float32                       `json:"discount_value"`
	Expires         *int                           `json:"expires"`
	Status          string                         `json:"status"`
	CanOrder        bool                           `json:"can_order"`
	CanReserve      bool                           `json:"can_reserve"`
}

type BusinessesAllDTO struct {
	Id              int      `json:"id"`
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	DiscountPercent *int     `json:"discount_percent"`
	Value           *float32 `json:"value"`
	DiscountValue   *float32 `json:"discount_value"`
	Status          string   `json:"status"`
}

type BusinessForUpdateDTO struct {
	Name           string
	ProvinceId     int
	DistrictId     int
	Phone          string
	DescriptionId  int
	CategoryId     *int
	SubcategoryIds []int
	OpensTime      string
	ClosesTime     string
	Expires        int
	Value          *float32
	CanOrder       bool
	CanReserve     bool
}

type Index struct {
	NewCreated    []IndexBusinesses `json:"new_created"`
	WithDiscounts []IndexBusinesses `json:"with_discounts"`
}

type IndexBusinesses struct {
	Id              int      `json:"id"`
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	DiscountPercent *int     `json:"discount_percent"`
	Value           *float32 `json:"value"`
	DiscountValue   *float32 `json:"discount_value"`
}

type DictionaryDTO struct {
	Tm string `json:"tm" binding:"required"`
	Ru string `json:"ru" binding:"required"`
	En string `json:"en" binding:"required"`
}

type AllAndSum struct {
	Businesses *[]BusinessesAllDTO `json:"businesses"`
	Count      int                 `json:"count"`
}

type UpdateStatus struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}
