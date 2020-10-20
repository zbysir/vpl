package parser

import (
	"fmt"
	"github.com/tdewolff/parse/v2"
	"github.com/zbysir/vpl/internal/lib/log"
	"github.com/zbysir/vpl/internal/lib/parse/html"
	"io"
	"strings"
)

// NodeType is the type of a Node.
type NodeType uint32

const (
	ErrorNode NodeType = iota
	TextNode
	DocumentNode
	ElementNode
	CommentNode
	DoctypeNode
	scopeMarkerNode
	// Root节点只是一个虚拟节点, 不渲染自己, 直接渲染子级
	RootNode
)

type Node struct {
	NodeType NodeType
	Tag      string
	Text     string // value of TextNode
	Attrs    []Attr
	Parent   *Node
	Child    []*Node
}

type Attr struct {
	Key   string
	Value string
}

func (p *Node) AddChild(n *Node) {
	p.Child = append(p.Child, n)
	n.Parent = p
	return
}

func (p *Node) AddBor(n *Node) {
	p.Parent.AddChild(n)

	return
}

func (p *Node) Add(n *Node) {
	// 如果是容器, 则往里添加, 否则添加为兄弟节点
	if p.NodeType == ElementNode || p.NodeType == RootNode {
		switch p.Tag {
		case "input", "br", "img", "meta":
			p.AddBor(n)
		default:
			p.AddChild(n)
		}
	} else {
		p.AddBor(n)
	}
	return
}

func (p *Node) GetParent(deep int) *Node {
	curr := p
	for i := 0; i < deep; i++ {
		if curr.Parent == nil {
			return curr
		}
		curr = curr.Parent
	}

	return curr
}

// 单标签, 在渲染和解析为节点树会使用.
var VoidElements = map[string]bool{
	"area":   true,
	"base":   true,
	"br":     true,
	"col":    true,
	"embed":  true,
	"hr":     true,
	"img":    true,
	"input":  true,
	"keygen": true,
	"link":   true,
	"meta":   true,
	"param":  true,
	"source": true,
	"track":  true,
	"wbr":    true,
}

// Close当前节点, 返回父级节点
func (p *Node) Close(tag string) *Node {
	if p.NodeType == ElementNode {
		// case for '<p><input></p>'
		if _, void := VoidElements[p.Tag]; void {
			if p.Tag != tag {
				return p.GetParent(2)
			} else {
				return p.GetParent(1)
			}
		} else {
			// 如果上一层的tag不是当前关闭的tag
			// 则说明有误: '<div>12311</span>'
			// 则忽略关闭: Closing tag matches nothing
			if p.Tag != tag {
				log.Warningf("bad close: currTag: %s, wantTag: %s", tag, p.Tag)
				return p
			} else {
				return p.GetParent(1)
			}
		}
	} else if p.NodeType == TextNode {
		// case for '<p>Title</p>'

		// 如果上一层的tag不是当前关闭的tag
		// 则说明有误: '<input>12311</input>'
		// 则忽略关闭: Closing tag matches nothing
		if p.Parent.Tag != tag {
			return p
		} else {
			// 否则关闭父级
			return p.GetParent(2)
		}

	}
	return nil
}

func (p *Node) NicePrint(lev int) string {
	s := strings.Repeat(" ", lev)
	switch p.NodeType {
	case ElementNode:
		s += fmt.Sprintf("<%v %+v>\n", p.Tag, p.Attrs)

		for _, v := range p.Child {
			s += fmt.Sprintf("%s", v.NicePrint(lev+1))
		}
	case TextNode:
		s += fmt.Sprintf("%s\n", p.Text)
	case CommentNode:
		s += fmt.Sprintf("%s\n", p.Text)
	case DoctypeNode:
		s += fmt.Sprintf("%s\n", p.Text)
	}

	return s
}

type NodeParser struct {
}

func NewNodeParser() *NodeParser {
	return &NodeParser{
	}
}

const whitespace = " \t\r\n\f"

func (p *NodeParser) Parse(l *html.Lexer) (node *Node, err error) {
	rootNode := &Node{
		NodeType: RootNode,
		Tag:      "",
		Text:     "",
		Attrs:    nil,
		Parent:   nil,
	}
	currNode := rootNode

	for {
		tt, data := l.Next()
		switch tt {
		case html.ErrorToken:
			// error or EOF set in l.Err()
			err = l.Err()
			if err != nil {
				if err == io.EOF {
					err = nil
				}
			}
			return rootNode, err
		case html.StartTagToken:
			tag := string(l.Text())
			nn := &Node{
				NodeType: ElementNode,
				Tag:      tag,
				Attrs:    nil,
				Child:    nil,
			}

			currNode.Add(nn)
			currNode = nn
		case html.StartTagCloseToken:
		case html.EndTagToken:
			tag := string(l.Text())

			currNode = currNode.Close(tag)

		case html.CommentToken:
			nn := &Node{
				NodeType: CommentNode,
				Tag:      "",
				Text:     byte2str(data),
				Attrs:    nil,
				Child:    nil,
			}
			currNode.Add(nn)
			currNode = nn
		case html.TextToken:
			text := strings.Trim(byte2str(data), whitespace)
			if len(text) == 0 {
				break
			}

			nn := &Node{
				NodeType: TextNode,
				Tag:      "",
				Text:     text,
				Attrs:    nil,
				Child:    nil,
			}
			currNode.Add(nn)
			currNode = nn
		case html.AttributeToken:
			// 删除引号
			attrVal := byte2str(l.AttrVal())
			if strings.HasPrefix(attrVal, `"`) && strings.HasSuffix(attrVal, `"`) {
				attrVal = attrVal[1 : len(attrVal)-1]
			} else if strings.HasPrefix(attrVal, `'`) && strings.HasSuffix(attrVal, `'`) {
				attrVal = attrVal[1 : len(attrVal)-1]
			}

			currNode.Attrs = append(currNode.Attrs, Attr{
				Key:   byte2str(l.Text()),
				Value: attrVal,
			})
		case html.StartTagVoidToken:
			currNode = currNode.Parent
		case html.DoctypeToken:
			nn := &Node{
				NodeType: DoctypeNode,
				Tag:      "",
				Text:     byte2str(data),
				Attrs:    nil,
				Child:    nil,
			}
			currNode.Add(nn)
			currNode = nn
		default:
			log.Infof("xxx %s: %s %v", tt, l.Text(), l.AttrVal())
		}
	}
}

func ParseHtml(str string) (nt *Node, err error) {
	l := html.NewLexer(parse.NewInputString(str))
	return NewNodeParser().Parse(l)
}

func byte2str(bs []byte) string {
	return string(bs)
}
func str2byte(s string) []byte {
	return []byte(s)
}
