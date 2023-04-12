%{
package deepql

import (
  "time"
)
%}

%start root

%union {
    root RootExpr
    groupOperation GroupOperation
    coalesceOperation CoalesceOperation

    snapshotExpression SnapshotExpression
    snapshotPipelineExpression SnapshotExpression
    wrappedSnapshotPipeline Pipeline
    snapshotPipeline Pipeline
    snapshotFilter SnapshotFilter
    scalarFilter ScalarFilter
    scalarFilterOperation Operator

    scalarPipelineExpressionFilter ScalarFilter
    scalarPipelineExpression ScalarExpression
    scalarExpression ScalarExpression
    wrappedScalarPipeline Pipeline
    scalarPipeline Pipeline
    aggregate Aggregate

    fieldExpression FieldExpression
    static Static
    intrinsicField Attribute
    attributeField Attribute

    binOp       Operator
    staticInt   int
    staticStr   string
    staticFloat float64
    staticDuration time.Duration
}

%type <RootExpr> root
%type <groupOperation> groupOperation
%type <coalesceOperation> coalesceOperation

%type <snapshotExpression> snapshotExpression
%type <snapshotPipelineExpression> snapshotPipelineExpression
%type <wrappedSnapshotPipeline> wrappedSnapshotPipeline
%type <snapshotPipeline> snapshotPipeline
%type <snapshotFilter> snapshotFilter
%type <scalarFilter> scalarFilter
%type <scalarFilterOperation> scalarFilterOperation

%type <scalarPipelineExpressionFilter> scalarPipelineExpressionFilter
%type <scalarPipelineExpression> scalarPipelineExpression
%type <scalarExpression> scalarExpression
%type <wrappedScalarPipeline> wrappedScalarPipeline
%type <scalarPipeline> scalarPipeline
%type <aggregate> aggregate

%type <fieldExpression> fieldExpression
%type <static> static
%type <intrinsicField> intrinsicField
%type <attributeField> attributeField

%token <staticStr>      IDENTIFIER STRING
%token <staticInt>      INTEGER
%token <staticFloat>    FLOAT
%token <staticDuration> DURATION
%token <val>            DOT OPEN_BRACE CLOSE_BRACE OPEN_PARENS CLOSE_PARENS
                        NIL TRUE FALSE
                        IDURATION NAME
                        RESOURCE_DOT
                        COUNT AVG MAX MIN SUM
                        BY COALESCE
                        END_ATTRIBUTE

// Operators are listed with increasing precedence.
%left <binOp> PIPE
%left <binOp> AND OR
%left <binOp> EQ NEQ LT LTE GT GTE NRE RE DESC TILDE
%left <binOp> ADD SUB
%left <binOp> NOT
%left <binOp> MUL DIV MOD
%right <binOp> POW
%%

// **********************
// Pipeline
// **********************
root:
    snapshotPipeline                             { yylex.(*lexer).expr = newRootExpr($1) }
  | snapshotPipelineExpression                   { yylex.(*lexer).expr = newRootExpr($1) }
  | scalarPipelineExpressionFilter               { yylex.(*lexer).expr = newRootExpr($1) }
  ;

// **********************
// snapshot Expressions
// **********************
snapshotPipelineExpression: // shares the same operators as snapshotExpression. split out for readability
    OPEN_PARENS snapshotPipelineExpression CLOSE_PARENS            { $$ = $2 }
  | snapshotPipelineExpression AND   snapshotPipelineExpression    { $$ = newSnapshotOperation(OpSnapshotAnd, $1, $3) }
  | snapshotPipelineExpression GT    snapshotPipelineExpression    { $$ = newSnapshotOperation(OpSnapshotChild, $1, $3) }
  | snapshotPipelineExpression DESC  snapshotPipelineExpression    { $$ = newSnapshotOperation(OpSnapshotDescendant, $1, $3) }
  | snapshotPipelineExpression OR    snapshotPipelineExpression    { $$ = newSnapshotOperation(OpSnapshotUnion, $1, $3) }
  | snapshotPipelineExpression TILDE snapshotPipelineExpression    { $$ = newSnapshotOperation(OpSnapshotSibling, $1, $3) }
  | wrappedSnapshotPipeline                                        { $$ = $1 }
  ;

wrappedSnapshotPipeline:
    OPEN_PARENS snapshotPipeline CLOSE_PARENS   { $$ = $2 }

snapshotPipeline:
    snapshotExpression                          { $$ = newPipeline($1) }
  | scalarFilter                                { $$ = newPipeline($1) }
  | groupOperation                              { $$ = newPipeline($1) }
  | snapshotPipeline PIPE snapshotExpression    { $$ = $1.addItem($3)  }
  | snapshotPipeline PIPE scalarFilter          { $$ = $1.addItem($3)  }
  | snapshotPipeline PIPE groupOperation        { $$ = $1.addItem($3)  }
  | snapshotPipeline PIPE coalesceOperation     { $$ = $1.addItem($3)  }
  ;

groupOperation:
    BY OPEN_PARENS fieldExpression CLOSE_PARENS { $$ = newGroupOperation($3) }
  ;

coalesceOperation:
    COALESCE OPEN_PARENS CLOSE_PARENS           { $$ = newCoalesceOperation() }
  ;

snapshotExpression: // shares the same operators as scalarPipelineExpression. split out for readability
    OPEN_PARENS snapshotExpression CLOSE_PARENS    { $$ = $2 }
  | snapshotExpression AND   snapshotExpression    { $$ = newSnapshotOperation(OpSnapshotAnd, $1, $3) }
  | snapshotExpression GT    snapshotExpression    { $$ = newSnapshotOperation(OpSnapshotChild, $1, $3) }
  | snapshotExpression DESC  snapshotExpression    { $$ = newSnapshotOperation(OpSnapshotDescendant, $1, $3) }
  | snapshotExpression OR    snapshotExpression    { $$ = newSnapshotOperation(OpSnapshotUnion, $1, $3) }
  | snapshotExpression TILDE snapshotExpression    { $$ = newSnapshotOperation(OpSnapshotSibling, $1, $3) }
  | snapshotFilter                                 { $$ = $1 }
  ;

snapshotFilter:
    OPEN_BRACE CLOSE_BRACE                      { $$ = newSnapshotFilter(NewStaticBool(true)) }
  | OPEN_BRACE fieldExpression CLOSE_BRACE      { $$ = newSnapshotFilter($2) }
  ;

scalarFilter:
    scalarExpression          scalarFilterOperation scalarExpression          { $$ = newScalarFilter($2, $1, $3) }
  ;

scalarFilterOperation:
    EQ     { $$ = OpEqual        }
  | NEQ    { $$ = OpNotEqual     }
  | LT     { $$ = OpLess         }
  | LTE    { $$ = OpLessEqual    }
  | GT     { $$ = OpGreater      }
  | GTE    { $$ = OpGreaterEqual }
  ;

// **********************
// Scalar Expressions
// **********************
scalarPipelineExpressionFilter:
    scalarPipelineExpression scalarFilterOperation scalarPipelineExpression { $$ = newScalarFilter($2, $1, $3) }
  | scalarPipelineExpression scalarFilterOperation static                   { $$ = newScalarFilter($2, $1, $3) }
  ;

scalarPipelineExpression: // shares the same operators as scalarExpression. split out for readability
    OPEN_PARENS scalarPipelineExpression CLOSE_PARENS        { $$ = $2 }
  | scalarPipelineExpression ADD scalarPipelineExpression    { $$ = newScalarOperation(OpAdd, $1, $3) }
  | scalarPipelineExpression SUB scalarPipelineExpression    { $$ = newScalarOperation(OpSub, $1, $3) }
  | scalarPipelineExpression MUL scalarPipelineExpression    { $$ = newScalarOperation(OpMult, $1, $3) }
  | scalarPipelineExpression DIV scalarPipelineExpression    { $$ = newScalarOperation(OpDiv, $1, $3) }
  | scalarPipelineExpression MOD scalarPipelineExpression    { $$ = newScalarOperation(OpMod, $1, $3) }
  | scalarPipelineExpression POW scalarPipelineExpression    { $$ = newScalarOperation(OpPower, $1, $3) }
  | wrappedScalarPipeline                                    { $$ = $1 }
  ;

wrappedScalarPipeline:
    OPEN_PARENS scalarPipeline CLOSE_PARENS    { $$ = $2 }
  ;

scalarPipeline:
    snapshotPipeline PIPE aggregate      { $$ = $1.addItem($3)  }
  ;

scalarExpression: // shares the same operators as scalarPipelineExpression. split out for readability
    OPEN_PARENS scalarExpression CLOSE_PARENS  { $$ = $2 }
  | scalarExpression ADD scalarExpression      { $$ = newScalarOperation(OpAdd, $1, $3) }
  | scalarExpression SUB scalarExpression      { $$ = newScalarOperation(OpSub, $1, $3) }
  | scalarExpression MUL scalarExpression      { $$ = newScalarOperation(OpMult, $1, $3) }
  | scalarExpression DIV scalarExpression      { $$ = newScalarOperation(OpDiv, $1, $3) }
  | scalarExpression MOD scalarExpression      { $$ = newScalarOperation(OpMod, $1, $3) }
  | scalarExpression POW scalarExpression      { $$ = newScalarOperation(OpPower, $1, $3) }
  | aggregate                                  { $$ = $1 }
  | static                                     { $$ = $1 }
  ;

aggregate:
    COUNT OPEN_PARENS CLOSE_PARENS                { $$ = newAggregate(aggregateCount, nil) }
  | MAX OPEN_PARENS fieldExpression CLOSE_PARENS  { $$ = newAggregate(aggregateMax, $3) }
  | MIN OPEN_PARENS fieldExpression CLOSE_PARENS  { $$ = newAggregate(aggregateMin, $3) }
  | AVG OPEN_PARENS fieldExpression CLOSE_PARENS  { $$ = newAggregate(aggregateAvg, $3) }
  | SUM OPEN_PARENS fieldExpression CLOSE_PARENS  { $$ = newAggregate(aggregateSum, $3) }
  ;

// **********************
// FieldExpressions
// **********************
fieldExpression:
    OPEN_PARENS fieldExpression CLOSE_PARENS { $$ = $2 }
  | fieldExpression ADD fieldExpression      { $$ = newBinaryOperation(OpAdd, $1, $3) }
  | fieldExpression SUB fieldExpression      { $$ = newBinaryOperation(OpSub, $1, $3) }
  | fieldExpression MUL fieldExpression      { $$ = newBinaryOperation(OpMult, $1, $3) }
  | fieldExpression DIV fieldExpression      { $$ = newBinaryOperation(OpDiv, $1, $3) }
  | fieldExpression MOD fieldExpression      { $$ = newBinaryOperation(OpMod, $1, $3) }
  | fieldExpression EQ fieldExpression       { $$ = newBinaryOperation(OpEqual, $1, $3) }
  | fieldExpression NEQ fieldExpression      { $$ = newBinaryOperation(OpNotEqual, $1, $3) }
  | fieldExpression LT fieldExpression       { $$ = newBinaryOperation(OpLess, $1, $3) }
  | fieldExpression LTE fieldExpression      { $$ = newBinaryOperation(OpLessEqual, $1, $3) }
  | fieldExpression GT fieldExpression       { $$ = newBinaryOperation(OpGreater, $1, $3) }
  | fieldExpression GTE fieldExpression      { $$ = newBinaryOperation(OpGreaterEqual, $1, $3) }
  | fieldExpression RE fieldExpression       { $$ = newBinaryOperation(OpRegex, $1, $3) }
  | fieldExpression NRE fieldExpression      { $$ = newBinaryOperation(OpNotRegex, $1, $3) }
  | fieldExpression POW fieldExpression      { $$ = newBinaryOperation(OpPower, $1, $3) }
  | fieldExpression AND fieldExpression      { $$ = newBinaryOperation(OpAnd, $1, $3) }
  | fieldExpression OR fieldExpression       { $$ = newBinaryOperation(OpOr, $1, $3) }
  | SUB fieldExpression                      { $$ = newUnaryOperation(OpSub, $2) }
  | NOT fieldExpression                      { $$ = newUnaryOperation(OpNot, $2) }
  | static                                   { $$ = $1 }
  | intrinsicField                           { $$ = $1 }
  | attributeField                           { $$ = $1 }
  ;

// **********************
// Statics
// **********************
static:
    STRING           { $$ = NewStaticString($1)           }
  | INTEGER          { $$ = NewStaticInt($1)              }
  | FLOAT            { $$ = NewStaticFloat($1)            }
  | TRUE             { $$ = NewStaticBool(true)           }
  | FALSE            { $$ = NewStaticBool(false)          }
  | NIL              { $$ = NewStaticNil()                }
  | DURATION         { $$ = NewStaticDuration($1)         }
  ;

intrinsicField:
    IDURATION      { $$ = NewIntrinsic(IntrinsicDuration)   }
  ;

attributeField:
    DOT IDENTIFIER END_ATTRIBUTE                      { $$ = NewAttribute($2)                                      }
  | RESOURCE_DOT IDENTIFIER END_ATTRIBUTE             { $$ = NewScopedAttribute(AttributeScopeResource, false, $2) }
  ;