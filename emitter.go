package main

import (
	"github.com/kamichidu/go-jclass"
)

type emitter interface {
	Emit(*jclass.JavaClass)
}

type typeInfo struct {
	Package      string        `json:"package"`
	SimpleName   string        `json:"simpleName"`
	TypeKind     string        `json:"typeKind"`
	AccessFlags  []string      `json:"accessFlags"`
	InnerClasses []*typeInfo   `json:"innerClasses"`
	Fields       []*fieldInfo  `json:"fields"`
	Methods      []*methodInfo `json:"methods"`
}

func newTypeInfoFromJavaClass(class *jclass.JavaClass) *typeInfo {
	info := new(typeInfo)
	info.Package = class.PackageName()
	info.SimpleName = class.SimpleName()
	info.AccessFlags = accessFlagsToStringSlice(class)
	if class.IsInterface() {
		info.TypeKind = "interface"
	} else if class.IsAnnotation() {
		info.TypeKind = "@interface"
	} else if class.IsEnum() {
		info.TypeKind = "enum"
	} else {
		info.TypeKind = "class"
	}
	info.InnerClasses = make([]*typeInfo, 0)
	info.Fields = make([]*fieldInfo, 0)
	for _, field := range class.Fields() {
		// only emit importable
		if field.IsPrivate() || !field.IsStatic() {
			continue
		}
		info.Fields = append(info.Fields, newFieldInfoFromJavaField(field))
	}
	info.Methods = make([]*methodInfo, 0)
	for _, method := range class.Methods() {
		// only emit importable
		if method.IsPrivate() || !method.IsStatic() {
			continue
		}
		info.Methods = append(info.Methods, newMethodInfoFromJavaMethod(method))
	}
	return info
}

func accessFlagsToStringSlice(flags jclass.AccessFlags) []string {
	symbols := make([]string, 0)
	if flags.IsPublic() {
		symbols = append(symbols, "public")
	} else if flags.IsProtected() {
		symbols = append(symbols, "protected")
	} else if flags.IsPrivate() {
		symbols = append(symbols, "private")
	}
	if flags.IsStatic() {
		symbols = append(symbols, "static")
	}
	if flags.IsFinal() {
		symbols = append(symbols, "final")
	} else if flags.IsAbstract() {
		symbols = append(symbols, "abstract")
	}
	return symbols
}

type fieldInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	AccessFlags []string `json:"accessFlags"`
}

func newFieldInfoFromJavaField(field *jclass.JavaField) *fieldInfo {
	info := new(fieldInfo)
	info.Name = field.Name()
	info.Type = field.Type()
	info.AccessFlags = accessFlagsToStringSlice(field)
	return info
}

type methodInfo struct {
	Name           string   `json:"name"`
	ReturnType     string   `json:"returnType"`
	ParameterTypes []string `json:"parameterTypes"`
	AccessFlags    []string `json:"accessFlags"`
}

func newMethodInfoFromJavaMethod(method *jclass.JavaMethod) *methodInfo {
	info := new(methodInfo)
	info.Name = method.Name()
	info.ReturnType = method.ReturnType()
	info.ParameterTypes = method.ParameterTypes()
	info.AccessFlags = accessFlagsToStringSlice(method)
	return info
}
