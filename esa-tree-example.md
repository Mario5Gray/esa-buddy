## SYSTEM

You are a Treesiter Query Assistant specializing in generating and testing Tree-sitter textobject queries. Your expertise includes:

1. Understanding Tree-sitter syntax and query patterns
2. Creating queries for different programming languages
3. Testing and debugging Tree-sitter queries
4. Understanding Abstract Syntax Tree (AST) structures

When a user asks for a query:
1. Generate a comprehensive Tree-sitter query based on their requirements
2. Save the user's code to a temporary file
3. Use tree-sitter-debugger to view the Tree-sitter parse tree of the code
4. Test the query against the code using tree-sitter-debugger
5. Provide the results along with explanations

Supported languages:
 - bash
 - c
 - cpp
 - css
 - go
 - html
 - java
 - javascript
 - js
 - php
 - py
 - python
 - ruby
 - rust
 - ts
 - tsx
 - typescript
 - yaml
 - yml

Example:
<user_query>
I need a query to select conditional statement in golang
</user_query>

<response>
(if_statement
  consequence: (block) @conditional.inner) @conditional.outer

(if_statement
  alternative: (block) @conditional.inner)? @conditional.outer

(expression_switch_statement
  (expression_case) @conditional.inner) @conditional.outer

(type_switch_statement
  (type_case) @conditional.inner) @conditional.outer

(select_statement
  (communication_case) @conditional.inner) @conditional.outer
</response>

Always remember to:
- View the tree structure first to understand the syntax nodes
- Test queries thoroughly before presenting results
- Explain the query and how it matches the code
- Suggest improvements or alternatives if the query doesn't work as expected
- Keep the query simple and generic, do not be too specific. See the examples above
- Only provide simple inner and outer textobjects
- Give just the queries at the end of the session
- Never include comments in your query


## USER

golang loop textobj

## ASSISTANT

I'll create Tree-sitter textobject queries for loops in Golang. First, let's understand the structure of different loop types in Go by examining their parse tree structure.

**Tool Calls**


Tool: `show_tree_structure` (ID: `tooluse_PzOeCujKQ8iAgef4XeC3UA`)

**`language`**: go


**`content`**
```
package main

func main() {
    // for loop
    for i := 0; i < 10; i++ {
        println(i)
    }
    
    // while-style loop
    j := 0
    for j < 10 {
        println(j)
        j++
    }
    
    // infinite loop
    for {
        break
    }
    
    // range loop
    nums := []int{1, 2, 3}
    for index, value := range nums {
        println(index, value)
    }
}
```


## TOOL

**Tool Result** (for call `tooluse_PzOeCujKQ8iAgef4XeC3UA`, tool `show_tree_structure`):

```
Command: tree-sitter-debugger --lang go

Output: 
(source_file
  (package_clause
    "package"
    (package_identifier "main")
  )
  "\n\n"
  (function_declaration
    "func"
    (identifier "main")
    (parameter_list
      "("
      ")"
    )
    (block
      "{"
      (comment "// for loop")
      (for_statement
        "for"
        (for_clause
          (short_var_declaration
            (expression_list
              (identifier "i")
            )
            ":="
            (expression_list
              (int_literal "0")
            )
          )
          ";"
          (binary_expression
            (identifier "i")
            "<"
            (int_literal "10")
          )
          ";"
          (inc_statement
            (identifier "i")
            "++"
          )
        )
        (block
          "{"
          (expression_statement
            (call_expression
              (identifier "println")
              (argument_list
                "("
                (identifier "i")
                ")"
              )
            )
          )
          "\n"
          "}"
        )
      )
      "\n"
      (comment "// while-style loop")
      (short_var_declaration
        (expression_list
          (identifier "j")
        )
        ":="
        (expression_list
          (int_literal "0")
        )
      )
      "\n"
      (for_statement
        "for"
        (binary_expression
          (identifier "j")
          "<"
          (int_literal "10")
        )
        (block
          "{"
          (expression_statement
            (call_expression
              (identifier "println")
              (argument_list
                "("
                (identifier "j")
                ")"
              )
            )
          )
          "\n"
          (inc_statement
            (identifier "j")
            "++"
          )
          "\n"
          "}"
        )
      )
      "\n"
      (comment "// infinite loop")
      (for_statement
        "for"
        (block
          "{"
          (break_statement
            "break"
          )
          "\n"
          "}"
        )
      )
      "\n"
      (comment "// range loop")
      (short_var_declaration
        (expression_list
          (identifier "nums")
        )
        ":="
        (expression_list
          (composite_literal
            (slice_type
              "["
              "]"
              (type_identifier "int")
            )
            (literal_value
              "{"
              (literal_element
                (int_literal "1")
              )
              ","
              (literal_element
                (int_literal "2")
              )
              ","
              (literal_element
                (int_literal "3")
              )
              "}"
            )
          )
        )
      )
      "\n"
      (for_statement
        "for"
        (range_clause
          (expression_list
            (identifier "index")
            ","
            (identifier "value")
          )
          ":="
          "range"
          (identifier "nums")
        )
        (block
          "{"
          (expression_statement
            (call_expression
              (identifier "println")
              (argument_list
                "("
                (identifier "index")
                ","
                (identifier "value")
                ")"
              )
            )
          )
          "\n"
          "}"
        )
      )
      "\n"
      "}"
    )
  )
)
```

## ASSISTANT

Now that I can see the parse tree structure for various loop types in Go, I'll create textobject queries for them. In Go, there are several types of loops:

1. Standard for loops (with init, condition, update)
2. While-style loops (just condition)
3. Infinite loops (no condition)
4. Range-based loops (for ... range)

Let's create queries for these loops and test them:

**Tool Calls**


Tool: `run_query` (ID: `tooluse_yRje5DbmRlq_KAmPXCzM-Q`)

**`language`**: go


**`content`**
```
package main

func main() {
    // for loop
    for i := 0; i < 10; i++ {
        println(i)
    }
    
    // while-style loop
    j := 0
    for j < 10 {
        println(j)
        j++
    }
    
    // infinite loop
    for {
        break
    }
    
    // range loop
    nums := []int{1, 2, 3}
    for index, value := range nums {
        println(index, value)
    }
}
```


**`query`**
```
(for_statement
  (block) @loop.inner) @loop.outer
```


## TOOL

**Tool Result** (for call `tooluse_yRje5DbmRlq_KAmPXCzM-Q`, tool `run_query`):

```
Command: tree-sitter-debugger --lang go --query '(for_statement (block) @loop.inner) @loop.outer'

Output: 
@loop.outer
start: 5:4
end: 7:5
content:
for i := 0; i < 10; i++ {
        println(i)
    }

@loop.inner
start: 5:28
end: 7:5
content:
{
        println(i)
    }


@loop.outer
start: 11:4
end: 14:5
content:
for j < 10 {
        println(j)
        j++
    }

@loop.inner
start: 11:15
end: 14:5
content:
{
        println(j)
        j++
    }


@loop.outer
start: 17:4
end: 19:5
content:
for {
        break
    }

@loop.inner
start: 17:8
end: 19:5
content:
{
        break
    }


@loop.outer
start: 23:4
end: 25:5
content:
for index, value := range nums {
        println(index, value)
    }

@loop.inner
start: 23:35
end: 25:5
content:
{
        println(index, value)
    }
```

## ASSISTANT

Great! This basic query successfully matches all types of for loops in Go. The query:

```
(for_statement
  (block) @loop.inner) @loop.outer
```

Captures:
- `@loop.outer`: The entire for statement including the `for` keyword, any conditions or clauses, and the code block
- `@loop.inner`: Just the block (code between curly braces)

Let's refine this to be more specific for different loop types:

**Tool Calls**


Tool: `run_query` (ID: `tooluse_V0pcEL48QmGJVkFA2WbYEA`)

**`language`**: go


**`content`**
```
package main

func main() {
    // for loop
    for i := 0; i < 10; i++ {
        println(i)
    }
    
    // while-style loop
    j := 0
    for j < 10 {
        println(j)
        j++
    }
    
    // infinite loop
    for {
        break
    }
    
    // range loop
    nums := []int{1, 2, 3}
    for index, value := range nums {
        println(index, value)
    }
}
```


**`query`**
```
(for_statement
  (for_clause)
  (block) @for_loop.inner) @for_loop.outer

(for_statement
  (range_clause)
  (block) @range_loop.inner) @range_loop.outer

(for_statement
  condition: (_) 
  (block) @condition_loop.inner) @condition_loop.outer

(for_statement
  (block) @infinite_loop.inner) @infinite_loop.outer
```


## TOOL

**Tool Result** (for call `tooluse_V0pcEL48QmGJVkFA2WbYEA`, tool `run_query`):

```
**ERROR:** Error: exit status 1
Command: tree-sitter-debugger --lang go --query '(for_statement (for_clause) (block) @for_loop.inner) @for_loop.outer (for_statement (range_clause) (block) @range_loop.inner) @range_loop.outer (for_statement condition: (_) (block) @condition_loop.inner) @condition_loop.outer (for_statement (block) @infinite_loop.inner) @infinite_loop.outer'
Output: Error executing query: invalid query: invalid unknown at line 1 column 0
condition: (_) (block) @condition_loop.inner) @condition_loop.outer (for_statement (block) @infinite_loop.inner) @infinite_loop.outer
^

```

## ASSISTANT

I see the error in my query. Let me correct it by removing the field name that isn't in the AST:

**Tool Calls**


Tool: `run_query` (ID: `tooluse_cMmgip99SD2LUxp7CVf7pA`)

**`content`**
```
package main

func main() {
    // for loop
    for i := 0; i < 10; i++ {
        println(i)
    }
    
    // while-style loop
    j := 0
    for j < 10 {
        println(j)
        j++
    }
    
    // infinite loop
    for {
        break
    }
    
    // range loop
    nums := []int{1, 2, 3}
    for index, value := range nums {
        println(index, value)
    }
}
```


**`query`**
```
(for_statement
  (for_clause)
  (block) @for_loop.inner) @for_loop.outer

(for_statement
  (range_clause)
  (block) @range_loop.inner) @range_loop.outer
```


**`language`**: go


## TOOL

**Tool Result** (for call `tooluse_cMmgip99SD2LUxp7CVf7pA`, tool `run_query`):

```
Command: tree-sitter-debugger --lang go --query '(for_statement (for_clause) (block) @for_loop.inner) @for_loop.outer (for_statement (range_clause) (block) @range_loop.inner) @range_loop.outer'

Output: 
@for_loop.outer
start: 5:4
end: 7:5
content:
for i := 0; i < 10; i++ {
        println(i)
    }

@for_loop.inner
start: 5:28
end: 7:5
content:
{
        println(i)
    }


@range_loop.outer
start: 23:4
end: 25:5
content:
for index, value := range nums {
        println(index, value)
    }

@range_loop.inner
start: 23:35
end: 25:5
content:
{
        println(index, value)
    }
```

## ASSISTANT

Let's add the remaining loop types:

**Tool Calls**


Tool: `run_query` (ID: `tooluse_ZaODeiiJQm65YdtVH9NLXA`)

**`language`**: go


**`content`**
```
package main

func main() {
    // for loop
    for i := 0; i < 10; i++ {
        println(i)
    }
    
    // while-style loop
    j := 0
    for j < 10 {
        println(j)
        j++
    }
    
    // infinite loop
    for {
        break
    }
    
    // range loop
    nums := []int{1, 2, 3}
    for index, value := range nums {
        println(index, value)
    }
}
```


**`query`**
```
(for_statement
  (_) @_cond
  (block) @loop.inner) @loop.outer

(for_statement
  (block) @loop.inner) @loop.outer
```


## TOOL

**Tool Result** (for call `tooluse_ZaODeiiJQm65YdtVH9NLXA`, tool `run_query`):

```
Command: tree-sitter-debugger --lang go --query '(for_statement (_) @_cond (block) @loop.inner) @loop.outer (for_statement (block) @loop.inner) @loop.outer'

Output: 
@loop.outer
start: 5:4
end: 7:5
content:
for i := 0; i < 10; i++ {
        println(i)
    }

@_cond
start: 5:8
end: 5:27
content:
i := 0; i < 10; i++

@loop.inner
start: 5:28
end: 7:5
content:
{
        println(i)
    }


@loop.outer
start: 5:4
end: 7:5
content:
for i := 0; i < 10; i++ {
        println(i)
    }

@loop.inner
start: 5:28
end: 7:5
content:
{
        println(i)
    }


@loop.outer
start: 11:4
end: 14:5
content:
for j < 10 {
        println(j)
        j++
    }

@_cond
start: 11:8
end: 11:14
content:
j < 10

@loop.inner
start: 11:15
end: 14:5
content:
{
        println(j)
        j++
    }


@loop.outer
start: 11:4
end: 14:5
content:
for j < 10 {
        println(j)
        j++
    }

@loop.inner
start: 11:15
end: 14:5
content:
{
        println(j)
        j++
    }


@loop.outer
start: 17:4
end: 19:5
content:
for {
        break
    }

@loop.inner
start: 17:8
end: 19:5
content:
{
        break
    }


@loop.outer
start: 23:4
end: 25:5
content:
for index, value := range nums {
        println(index, value)
    }

@_cond
start: 23:8
end: 23:34
content:
index, value := range nums

@loop.inner
start: 23:35
end: 25:5
content:
{
        println(index, value)
    }


@loop.outer
start: 23:4
end: 25:5
content:
for index, value := range nums {
        println(index, value)
    }

@loop.inner
start: 23:35
end: 25:5
content:
{
        println(index, value)
    }
```

## ASSISTANT

I notice there are some duplications in the captures because both patterns are matching. Let me create a more precise set of queries for different loop types:

**Tool Calls**


Tool: `run_query` (ID: `tooluse_-0K6bGbHRIKQT6DbpOEvYw`)

**`language`**: go


**`content`**
```
package main

func main() {
    // for loop
    for i := 0; i < 10; i++ {
        println(i)
    }
    
    // while-style loop
    j := 0
    for j < 10 {
        println(j)
        j++
    }
    
    // infinite loop
    for {
        break
    }
    
    // range loop
    nums := []int{1, 2, 3}
    for index, value := range nums {
        println(index, value)
    }
}
```


**`query`**
```
(for_statement
  (block) @loop.inner) @loop.outer
```


## TOOL

**Tool Result** (for call `tooluse_-0K6bGbHRIKQT6DbpOEvYw`, tool `run_query`):

```
Command: tree-sitter-debugger --lang go --query '(for_statement (block) @loop.inner) @loop.outer'

Output: 
@loop.outer
start: 5:4
end: 7:5
content:
for i := 0; i < 10; i++ {
        println(i)
    }

@loop.inner
start: 5:28
end: 7:5
content:
{
        println(i)
    }


@loop.outer
start: 11:4
end: 14:5
content:
for j < 10 {
        println(j)
        j++
    }

@loop.inner
start: 11:15
end: 14:5
content:
{
        println(j)
        j++
    }


@loop.outer
start: 17:4
end: 19:5
content:
for {
        break
    }

@loop.inner
start: 17:8
end: 19:5
content:
{
        break
    }


@loop.outer
start: 23:4
end: 25:5
content:
for index, value := range nums {
        println(index, value)
    }

@loop.inner
start: 23:35
end: 25:5
content:
{
        println(index, value)
    }
```

## ASSISTANT

Perfect! After analyzing all the loop types in Go, I can see that a simple query works well for all cases:

```
(for_statement
  (block) @loop.inner) @loop.outer
```

This query captures all types of loops in Go:
- Traditional for loops (with initialization, condition, and increment)
- While-style loops (just a condition)
- Infinite loops (no condition)
- Range-based loops (using `range` keyword)

For each loop:
- `@loop.outer` captures the entire loop statement including the `for` keyword, any conditions/clauses, and the code block
- `@loop.inner` captures just the block content (the code between curly braces)

The query is simple and effective for all Go loop types. Since Go uses the same `for` keyword for all loop constructs and they all have the same basic structure in the AST, a single pattern works well.

Here's the final textobject query for Go loops:

```
(for_statement
  (block) @loop.inner) @loop.outer
```