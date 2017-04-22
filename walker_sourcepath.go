package main

import (
	"bufio"
	"os/exec"
	"strings"
)

type sourceWalker struct {
	Directory string
}

func (self *sourceWalker) Walk(c *ctx) error {
	ctags := exec.Command("ctags", "-f", "-", "--langmap=Java:.java", "--java-kinds=cgifmp", "--recurse=yes", "--extra=", "--sort=no", self.Directory)

	errReader, err := ctags.StderrPipe()
	if err != nil {
		c.Logger().Printf("Failed to open stderr pipe: %s", err)
		return nil
	}
	defer errReader.Close()

	outReader, err := ctags.StdoutPipe()
	if err != nil {
		c.Logger().Printf("Failed to open stdout pipe: %s", err)
		return nil
	}
	defer outReader.Close()

	if err = ctags.Start(); err != nil {
		c.Logger().Printf("Failed to create process: %s", err)
		return nil
	}

	go func() {
		r := bufio.NewScanner(errReader)
		for r.Scan() {
			c.Logger().Println(r.Text())
		}
		if err := r.Err(); err != nil {
			c.Logger().Printf("Can't read stderr pipe: %s", err)
		}
	}()

	var cTags classTags
	r := bufio.NewScanner(outReader)
	for r.Scan() {
		tags := self.parseTagLine(c, r.Text())
		// always a class tags starts by package tag (kind="p") when ctags with sort=no
		if kind, ok := tags.Tagfield["kind"]; ok && kind == "p" {
			if cTags != nil {
				c.Emitter().Emit(self.newTypeInfoFromClassTags(cTags))
			}
			cTags = make(classTags, 0)
		}
		cTags = append(cTags, tags)
	}
	if cTags != nil {
		c.Emitter().Emit(self.newTypeInfoFromClassTags(cTags))
	}
	if err = r.Err(); err != nil {
		c.Logger().Printf("Can't read stdout pipe: %s", err)
	}
	return ctags.Wait()
}

type classTags []*tags

func (self classTags) PackageName() string {
	tag := self.First("p")
	if tag != nil {
		return tag.Tagname
	} else {
		return ""
	}
}

func (self classTags) TypeKind() string {
	kinds := [][]string{
		{"c", "class"},
		{"i", "interface"},
		{"g", "enum"},
	}
	for _, kind := range kinds {
		tag := self.First(kind[0])
		if tag != nil {
			return kind[1]
		}
	}
	return "class"
}

func (self classTags) SimpleName() string {
	kinds := [][]string{
		{"c", "class"},
		{"i", "interface"},
		{"g", "enum"},
	}
	for _, kind := range kinds {
		tag := self.First(kind[0])
		if tag != nil {
			return tag.Tagname
		}
	}
	return ""
}

func (self classTags) AccessFlags() []string {
	for _, kind := range []string{"c", "i", "g"} {
		tag := self.First(kind)
		if tag != nil {
			return tag.AccessFlags()
		}
	}
	return make([]string, 0)
}

func (self classTags) First(kind string) *tags {
	for _, tag := range self {
		if v, ok := tag.Tagfield["kind"]; ok && v == kind {
			return tag
		}
	}
	return nil
}

func (self classTags) Find(kind string) []*tags {
	tagList := make([]*tags, 0)
	for _, tag := range self {
		if v, ok := tag.Tagfield["kind"]; ok && v == kind {
			tagList = append(tagList, tag)
		}
	}
	return tagList
}

type tags struct {
	Tagname    string
	Tagfile    string
	Tagaddress string
	Tagfield   map[string]string
}

func (self *tags) AccessFlags() []string {
	tagaddress := self.Tagaddress
	// eliminate prefix /^, suffix $/"
	tagaddress = tagaddress[2 : len(tagaddress)-2]

	flags := make([]string, 0)
	for _, word := range strings.Split(tagaddress, " ") {
		switch word {
		case "public", "protected", "private", "static", "final", "abstract":
			flags = append(flags, word)
		}
	}
	return flags
}

func (self *tags) FieldType() string {
	words := strings.Split(self.Tagaddress, " ")
	var word string
	for len(words) > 0 {
		word, words = words[len(words)-1], words[:len(words)-1]
		if self.Tagname == word {
			return words[len(words)-1]
		}
	}
	return ""
}

func (self *tags) MethodReturnType() string {
	return "???"
}

func (self *tags) MethodParameterTypes() []string {
	return make([]string, 0)
}

// http://ctags.sourceforge.net/FORMAT
func (self *sourceWalker) parseTagLine(c *ctx, line string) *tags {
	items := strings.SplitN(line, "\t", 3)
	tagname, tagfile, rest := items[0], items[1], items[2]

	var tagaddress string
	var tagfield map[string]string
	if strings.Contains(rest, ";\"") {
		// has tagfield
		items = strings.SplitN(rest, ";\"", 2)
		tagaddress, rest = items[0], items[1]
		tagfield = self.parseTagfield(rest)
	} else {
		// no tagfield
		tagaddress = rest
		tagfield = make(map[string]string)
	}

	return &tags{
		Tagname:    tagname,
		Tagfile:    tagfile,
		Tagaddress: tagaddress,
		Tagfield:   tagfield,
	}
}

func (self *sourceWalker) parseTagfield(tagfield string) map[string]string {
	data := make(map[string]string)
	for _, item := range strings.Split(tagfield, "\t") {
		if strings.Contains(item, ":") {
			kv := strings.SplitN(item, ":", 2)
			data[kv[0]] = kv[1]
		} else {
			// omitted "kind:"
			data["kind"] = item
		}
	}
	return data
}

func (self *sourceWalker) newTypeInfoFromClassTags(tags classTags) *typeInfo {
	info := new(typeInfo)
	info.Package = tags.PackageName()
	info.SimpleName = tags.SimpleName()
	info.TypeKind = tags.TypeKind()
	info.AccessFlags = tags.AccessFlags()
	// only emit importable
	if stringSliceContains(info.AccessFlags, "private") {
		return nil
	}
	info.InnerClasses = make([]*typeInfo, 0)
	info.Fields = make([]*fieldInfo, 0)
	for _, fieldTag := range tags.Find("f") {
		fieldInfo := &fieldInfo{
			Name:        fieldTag.Tagname,
			Type:        fieldTag.FieldType(),
			AccessFlags: fieldTag.AccessFlags(),
		}
		// only emit importable
		if stringSliceContains(fieldInfo.AccessFlags, "private") {
			continue
		} else if !stringSliceContains(fieldInfo.AccessFlags, "static") {
			continue
		}
		info.Fields = append(info.Fields, fieldInfo)
	}
	info.Methods = make([]*methodInfo, 0)
	for _, methodTag := range tags.Find("m") {
		methodInfo := &methodInfo{
			Name:           methodTag.Tagname,
			AccessFlags:    methodTag.AccessFlags(),
			ReturnType:     methodTag.MethodReturnType(),
			ParameterTypes: methodTag.MethodParameterTypes(),
		}
		// only emit importable
		if stringSliceContains(methodInfo.AccessFlags, "private") {
			continue
		} else if !stringSliceContains(methodInfo.AccessFlags, "static") {
			continue
		}
		info.Methods = append(info.Methods, methodInfo)
	}
	return info
}

func stringSliceContains(a []string, s string) bool {
	for _, el := range a {
		if el == s {
			return true
		}
	}
	return false
}

// Memo:
// ctags -f - --langmap=Java:.java --java-kinds=cgifmp --recurse=yes --extra=q
// --java-kinds candidates are:
//   c  classes
//   e  enum constants
//   f  fields
//   g  enum types
//   i  interfaces
//   l  local variables [off]
//   m  methods
//   p  packages

var _ walker = (*sourceWalker)(nil)
