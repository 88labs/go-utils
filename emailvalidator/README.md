# emailvalidator

This is a Go implementation of an email validation library compatible with the RFC mode of [K-and-R/email_validator](https://github.com/K-and-R/email_validator).

## Features

- Validates email addresses according to RFC 5322 rules.
- Supports local-part validation with allowed special characters.
- Enforces domain name structure, including top-level domain (TLD) validation.
- Allows IPv4 and IPv6 address literals in the domain part.
- Rejects invalid email structures, such as consecutive dots, trailing/leading dots, and missing domain parts.
- Does not support internationalized domain names (IDN) or non-ASCII characters.

## Installation

```
go get github.com/88labs/go-utils/emailvalidator
```

## Usage

```
package main

import (
	"fmt"
	"github.com/88labs/go-utils/emailvalidator"
)

func main() {
	emails := []string{
		"user@example.com",
		"user@sub.example.co.jp",
		"user@[192.168.1.1]",
		"user@localhost",
		"invalid@-example.com",
		"invalid@example.123",
		"user",
	}

	for _, email := range emails {
		fmt.Printf("%s is valid? %t\n", email, emailvalidator.IsValid(email))
	}
}
```

## Validator Comparison

|| K-and-R/email_validator(rfc-mode)| emailvalidator | mail.ParseAddress(Standard library)|
|---------------------------------------------------------------------------|-------|-------|-------|
| user@example.com                                                          | true  | true  | true  |
| include-"-quotedouble@example.com                                         | true  | true  | false |
| bracketed-and-labeled-IPv6@[IPv6:abcd:ef01:1234:5678:9abc:def0:1234:5678] | true  | true  | false |
| end-with-dot.@invalid-characters-in-local.dev                             | false | false | false |
| start-with-ampersand@&invalid-characters-in-domain.dev                    | false | false | true  |
| test@umläut.com                                                           | true  | false | true  |


## Limitations

- This library does not support non-ASCII characters in the local or domain parts.
- Internationalized domain names (IDN) are not supported.
- Validation is not a full implementation of RFC 5322 but follows its practical subset.

## Acknowledgments

This implementation is inspired by the RFC mode of K-and-R/email_validator.
