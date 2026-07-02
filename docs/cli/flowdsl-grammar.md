# FlowDSL Grammar

```
File = Entry* .
Entry = Import | Workflow .
Import = "import" <string> .
Workflow = "workflow" <string> "{" WorkflowItem* "}" .
WorkflowItem = Param | Task | Attr .
Param = "param" <ident> ":" <ident> ("=" Value)? .
Value = String | <float> | <int> | Bool | List | Map | <ident> .
String = <string> .
Bool = ("true" | "false") .
List = "[" Value* "]" .
Map = "{" MapEntry* "}" .
MapEntry = MapKey ":" Value .
MapKey = <ident> | String .
Task = "task" <ident> "{" Attr* "}" .
Attr = <ident> ":" Value .
```
