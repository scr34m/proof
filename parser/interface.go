package parser

type Parser interface {
	Load(string) error
	Process() error
}
