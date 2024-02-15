# Languages specifics

As Deep supports a variety of languages the way the expressions need to be defined depends on the language. There are
also some caveats depending on the language being used.

!!! warning 
    In all cases it is possible to execute code that is potentially harmful to your application. It is therefore the
    responsibility of the user to ensure the expressions will not have an adverse effect on the application.

## Python

Python will evaluate the expressions using `eval` this allows you to execute any valid python code within the
expressions. This allows for a lot of power to collect data, however it also allows for some potential bad scenarios.
Where potentially harmful code can be executed. As a result we have tried to defend the user from some scenarios.

- Global values are not exposed to `eval`

## Java

In Java there is no equivalent of `eval` this means that to execute code we need to provide an evaluator. The default
evaluator is the '[Nashorn](https://en.wikipedia.org/wiki/Nashorn_(JavaScript_engine))' JavaScript engine. This was
chosen as it is shipped in the JDK, so we can reduce the size of the Deep agent. However, this evaluator is not
available
in all version of Java and as such it is possible that expressions will not work in when using Java.

!!! Note
    It is planned to release additional evaluators that can be used when Nashorn is not available.

## ColdFusion (Adobe/Lucee)

In ColdFusion expression will use the `Evaluate` function (or `evaluate` for Lucee) that is available on the page
context. This allows for ColdFusion expressions to be evaluated.
