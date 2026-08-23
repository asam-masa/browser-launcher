package appinfo

type Info struct {
	Name    string
	Version string
}

type Service struct {
	info Info
}

func NewService(name string, version string) Service {
	return Service{
		info: Info{
			Name:    name,
			Version: version,
		},
	}
}

func (s Service) Get() Info {
	return s.info
}
