# mpath
mpath is a tool similar to jsonpath, in that a query can be written and assessed against a data set, including the ability to run calculations and comparisons.

This tool relies heavily on the `reflect` package, and reflection at runtime can be expensive. In this case, the data is only 'reflected' upon as is absolutely necessary, which avoids using something like `json.Unmarshal` into a `map[string]any` or `[]any`, which recurses the entire object tree, regardless of whether the data in the whole tree is being used. 

This means that you can pass arbitrarily large objects into the `Do` method without it being inefficient.

One important thing to note is that all numbers are treated as decimals such that any arithmetic operations are run using decimal maths, rather than floating point maths that is not suitable for financial calculations.

For more in depth examples, see the test cases defined in `mpath_test.go`.


---


There are two main entry points:

```
func ParseString(ss string) (topOp Operation, err error)
```
ParseString converts the string representation of a query to a tree of `Operation` structs. 

`err` is returned as an error if the string does not parse properly.

`Operation` is an interface:
``` go
type Operation interface {
	Do(currentData, originalData any) (dataToUse any, err error)
	Parse(s *scanner, r rune) (nextR rune, err error)
	Sprint(depth int) (out string)
	Type() ot_OpType
}
```

Once one has an `Operation`, one can call the `Do` method by passing in base data into both parameters.

The `Operation` returned from `ParseString` can be any of the following built in structs:

### `opPath`
This is a 'path' through the data, and can start with either `$`, which represents the root of the data sent to the `Do` method, or `@`, which is used in filters to represent the data at that point in the path.

And example here is:
```
$.Tags[@.Contains("test")].First()
```
for the data:
``` json
{
    "name": "xyz",
    "tags": [
        "test123",
        "tes123",
        "xyz987",
        "999test"
    ]
}
```

That function will return the first `Tag` returned from a filtered list of `Tags` that contain the (case sensitive) string, "test". The returned value would be:
``` json
"test123"
```
Without the call the the `First` function, the result would be:
``` json
[
    "test123",
    "999test"
]
```


The `$` path identifier can also be used to provide data to functions as parameters, that are filled at runtime; example:
```
$.tags[@.HasPrefix($.name)]
```
which would return:
``` json
[
    "xyz987"
]
```

### `opPathIdent`

These are the named 'parts' of the path; example:
```
$.this.IS.a.Collection.First().OtherProperty
```

The `pathIdents` here would be:
```
- this
- IS
- a
- Collection
- OtherProperty
```

`pathIdents` are case insensitive.

### `opFilter`

To filter a collection, or to remove a struct from continuing along the chain of `Operations`, one can using an `opFilter` by using square brackets (`[` and `]`) at the start and end of a filter operation. The filter operation expects to be provided with a path starting with the `@` symbol, to represent the fields of the data being filtered, and must end as a boolean. 

This can be done either by providing a boolean field, or by using a boolean function (see `opFunction`).

Under the hood, an `opFilter` runs it's parameters as an `opLogicalOperation`, so one can use the `AND` or `OR` mode in filtering (`AND` is actually the default mode); example:

```
$.someCollection[AND,@.name.Equal("Bob Jones"),@.country.OneOf("Australia", "New Zealand")]
```

### `opLogicalOperation`

A logical operation is essentially just a collection of boolean operations. As logical operations return booleans themselves, they can be nested.

The first parameter to `opLogicalOperation` is the 'mode' of the operation, which can either be `AND` or `OR`. 

If this parameter is not provided, the system assumes that the 'mode' is `AND`. 

### `opFunction`

There are several functions available to be used, and they must be used **after** an `opIdent` or another `opFunction`.

- `Equal`
  -  Takes one parameter
  -  Tests whether the input and the parameter are the same

- `NotEqual`
  -  Takes one parameter
  -  Tests whether the input and the parameter are not the same

- `AnyOf`
  - Takes one or more parameters
  - Tests whether the input is equal to any of the parameters

Only for use with numbers:

- `Less`
  - Takes one parameter
  - Tests whether the input is less than the parameter

- `LessOrEqual`
  - Takes one parameter
  - Tests whether the input is less than or equal the parameter

- `Greater`
  - Takes one parameter
  - Tests whether the input is greater than the parameter

- `GreaterOrEqual`
  - Takes one parameter
  - Tests whether the input is greater than or equal the parameter

Only for use with strings:

- `Contains`
  - Takes one parameter
  - Tests whether the input contains the parameter

- `NotContains`
  - Takes one parameter
  - Tests whether the input does not contain the parameter

- `Prefix`
  - Takes one parameter
  - Tests whether the input starts with the parameter

- `NotPrefix`
  - Takes one parameter
  - Tests whether the input does not start with the parameter

- `Suffix`
  - Takes one parameter
  - Tests whether the input ends with the parameter

- `NotSuffix`
  - Takes one parameter
  - Tests whether the input does not end with the parameter

Only for use with arrays:

- `Count`
  - Takes no parameters
  - Returns the number of elements in the array

- `Any`
  - Takes no parameters
  - Tests whether the array has any elements

- `First`
  - Takes no parameters
  - Returns the first element in the array (if not empty)

- `Last`
  - Takes no parameters
  - Returns the last element in the array (if not empty)

- `Index`
  - Takes one parameter
  - Returns the element at the zero based index of the array, as defined by the parameter (if not empty)

Only for use with numbers or arrays of numbers:

- `Sum`
  - Takes no parameters or many
  - Returns the sum of the number(s) in the input and the number(s) in the parameters

- `Avg`
  - Takes no parameters or many
  - Returns the average of the number(s) in the input and the number(s) in the parameters

- `Min`
  - Takes no parameters or many
  - Returns the minimum of the number(s) in the input and the number(s) in the parameters

- `Max`
  - Takes no parameters or many
  - Returns the maximum of the number(s) in the input and the number(s) in the parameters

Only for use with numbers:

- `Add`
  - Takes one parameter
  - Adds the parameter to the input (e.g. `input + param`)

- `Sub`
  - Takes one parameter
  - Subtracts the parameter from the input (e.g. `input - param`)

- `Div`
  - Takes one parameter
  - Divides the input by the parameter (e.g. `input ÷ param`)

- `Mul`
  - Takes one parameter
  - Multiplies the input by the parameter (e.g. `input x param`)

- `Mod`
  - Takes one parameter
  - Returns the modulus of the input modulo the parameter (e.g. `input % param`)


### Future planned work:

- Provide the ability to pass in custom functions when initialising the package, and being able to call them by name.
- Provide the ability to define 'global variables' that can be set prior to the main operation, and then can be referred to by name. The plan here is to use the `#` character to define them, and to use them later. This will mean that variables can be used more efficiently. 
- Double check that we're not inefficiently converting decimals.