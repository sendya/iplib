package iplib

type Continent struct {
	Name   string // 大洲名称
	NameCN string // 大洲中文名称
}

// continentMapping 存储国家码到大洲代码的映射
var continentMapping = map[string]string{
	"CN": "AS", "JP": "AS", "KR": "AS", "IN": "AS", // 亚洲国家
	"US": "NA", "CA": "NA", "MX": "NA", // 北美洲
	"GB": "EU", "FR": "EU", "DE": "EU", "IT": "EU", // 欧洲
	"BR": "SA", "AR": "SA", // 南美洲
	"AU": "OC", "NZ": "OC", // 大洋洲
	"EG": "AF", "NG": "AF", // 非洲
	"AQ": "AN", // 南极洲
}

var continentNames = map[string]Continent{
	"AS": {"Asia", "亚洲"},
	"EU": {"Europe", "欧洲"},
	"NA": {"North America", "北美洲"},
	"SA": {"South America", "南美洲"},
	"AF": {"Africa", "非洲"},
	"OC": {"Oceania", "大洋洲"},
	"AN": {"Antarctica", "南极洲"},
}

// GetContinentByCountry 根据国家码获取大洲代码
func GetContinentByCountry(countryCode string) string {
	if continent, ok := continentMapping[countryCode]; ok {
		return continent
	}
	return "Unknown"
}

// GetContinentName 根据大洲代码获取大洲名称
func GetContinentName(continentCode string) Continent {
	if continent, ok := continentNames[continentCode]; ok {
		return continent
	}
	return Continent{"Unknown", "未知"}
}
