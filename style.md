# Style and conventions

## One sentence per line

To keep consistency throughout the Markdown files in the Open Container spec all files should be formatted one sentence per line.
This fixes two things: it makes diffing easier with git and it resolves fights about line wrapping length.
For example, this paragraph will span three lines in the Markdown source.

## Traditionally hex settings should use JSON integers, not JSON strings

For example, [`"classID": 1048577`][class-id] instead of `"classID": "0x100001"`.
The config JSON isn't enough of a UI to be worth jumping through string <-> integer hoops to support an 0x… form ([source][integer-over-hex]).

## Constant names should keep redundant prefixes

For example, `CAP_KILL` instead of `KILL` in [**`linux.capabilities`**][capabilities].
The redundancy reduction from removing the namespacing prefix is not useful enough to be worth trimming the upstream identifier ([source][keep-prefix]).

## Optional settings should not have pointer Go types

Because in many cases the Go default for the type is a no-op in the spec (sources [here][no-pointer-for-strings], [here][no-pointer-for-slices], and [here][no-pointer-for-boolean]).
The exceptions are entries where we need to distinguish between “not set” and “set to the Go default for that type” ([source][pointer-when-updates-require-changes]), and this decision should be made on a per-setting case.

## Examples

### Anchoring

For any given section that provides a notable example, it is ideal to have it denoted with [markdown headers][markdown-headers].
The level of header should be such that it is a subheader of the header it is an example of.

#### Example

```markdown
## Some Topic

### Some Subheader

#### Further Subheader

##### Example

To use Further Subheader, ...

### Example

To use Some Topic, ...

```

### Content

Where necessary, the values in the example can be empty or unset, but accommodate with comments regarding this intention.

Where feasible, the content and values used in an example should convey the fullest use of the data structures concerned.
Most commonly onlookers will intend to copy-and-paste a "working example".
If the intention of the example is to be a fully utilized example, rather than a copy-and-paste example, perhaps add a comment as such.

```markdown
### Example
```
```json
{
    "foo": null,
    "bar": ""
}
```

**vs.**

```markdown
### Example

Following is a fully populated example (not necessarily for copy/paste use)
```
```json
{
    "foo": [
        1,
        2,
        3
    ],
    "bar": "waffles",
    "bif": {
        "baz": "potatoes"
    }
}
```

[capabilities]: config-linux.md#capabilities
[class-id]: config-linux.md#network
[integer-over-hex]: https://github.com/opencontainers/runtime-spec/pull/267#r48360013
[keep-prefix]: https://github.com/opencontainers/runtime-spec/pull/159#issuecomment-138728337
[no-pointer-for-boolean]: https://github.com/opencontainers/runtime-spec/pull/290#r50296396
[no-pointer-for-slices]: https://github.com/opencontainers/runtime-spec/pull/316#r50782982
[no-pointer-for-strings]: https://github.com/opencontainers/runtime-spec/pull/653#issue-200439192
[pointer-when-updates-require-changes]: https://github.com/opencontainers/runtime-spec/pull/317#r50932706
[markdown-headers]: https://help.github.com/articles/basic-writing-and-formatting-syntax/#headings
