# Snapshots

Deeps primary feature is the ability to capture data at any point in your code. This allows you to gather any data at
any point to help speed up diagnosis of application issues.

The way Deep triggers controls its data collection is with '[Tracepoints](./tracepoints.md)'. A tracepoint is a
definition of a point of code (file and line number), with some customisations of how to collect data. The default
behaviour is to create a snapshot that contains the stack and variable data at that point in the code.

=== "API"
    
    ```json
    {
      "path": "some/file.py",
      "line_number": 22
    }
    ```

=== "DeepQL"

    ```
    snapshot{path="some/file.py" line=22}
    ```

## Data Collection

The data that is collected by a snapshot is controlled by the config of the tracepoint but can include:

 - Stack Frames - The list of frames that where executed for the application to get to the tracepoints location, this includes file name, class name, line number etc.
 - Variables - The variable data that is available at that point of the code, this includes local variables, method parameters (arguments), defining object (this/self)
 - Watches - User specified values that are evaluated at the point the code is executed

It is possible to customise the data collection by using plugins.

