# cloudwriter

cloudwriter is a implementation of io.Reader that ships data to AWS CloudWatch
 
## Example 
 
```
package main

import 
func main() {
    w, err := cloudwriter.New(nil, "sample-group", "sample-stream")
    io.WriteString("Hello World\n")
}
```