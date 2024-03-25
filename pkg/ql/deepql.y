%{
package ql

import (
  "time"
  "fmt"
)
%}

// define go type fields
%union{
	root RootExpr

	trigger trigger
	command command
	options []configOption

	staticInt   int
	staticStr   string
	staticFloat float64
	staticDuration time.Duration
	static Static

    	operator Operator
    	option configOption
    	fieldName string
}

// start at 'root; type
%start root

// tokenize items into fields
%token <staticStr> STRING IDENTIFIER TRIGGER COMMAND
%token <staticInt>      INTEGER
%token <staticFloat>    FLOAT
%token <staticDuration> DURATION

%token <val> OPEN_BRACE CLOSE_BRACE
             NIL TRUE FALSE DOT

// map go types to yacc types
%type <RootExpr> root

%type <trigger> trigger
%type <command> command

%type <fieldName> fieldName
%type <options> options
%type <option> option
%type <operator> operator
%type <static> static

%left <binOp> EQ NEQ LT LTE GT GTE
%%
root:
	trigger { yylex.(*lexer).expr = &RootExpr{trigger: &$1} }
	| command { yylex.(*lexer).expr = &RootExpr{command: &$1} }
	;

// allow creation of triggers
// log{ ... }
// metric{ ... }
// snapshot{ ... }
trigger:
	TRIGGER OPEN_BRACE CLOSE_BRACE { $$ = newTrigger($1, nil) }
	| TRIGGER OPEN_BRACE options CLOSE_BRACE { $$ = newTrigger($1, $3) }
	;

command:
	COMMAND OPEN_BRACE CLOSE_BRACE { $$ = newCommand($1, nil) }
	| COMMAND OPEN_BRACE options CLOSE_BRACE { $$ = newCommand($1, $3) }
        ;

options:
	option options { $$ = append($2, $1) }
	| option             { $$ = append($$, $1) }
	;

option:
	fieldName operator static { $$ = newConfigOption($2, $1, $3) }
	;

fieldName:
	IDENTIFIER { $$ = $1 }
	| IDENTIFIER DOT fieldName { $$ = fmt.Sprintf("%s.%s", $1, $3) }

// **********************
// Statics
// **********************
static:
	STRING             { $$ = NewStaticString($1)           }
	| INTEGER          { $$ = NewStaticInt($1)              }
	| FLOAT            { $$ = NewStaticFloat($1)            }
	| TRUE             { $$ = NewStaticBool(true)           }
	| FALSE            { $$ = NewStaticBool(false)          }
	| NIL              { $$ = NewStaticNil()                }
	| DURATION         { $$ = NewStaticDuration($1)         }
	;

operator:
	EQ       { $$ = OpEqual        }
	| NEQ    { $$ = OpNotEqual     }
	| LT     { $$ = OpLess         }
	| LTE    { $$ = OpLessEqual    }
	| GT     { $$ = OpGreater      }
	| GTE    { $$ = OpGreaterEqual }
	;
%%
