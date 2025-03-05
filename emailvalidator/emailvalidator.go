// Validates an email address based on RFC 5322-like rules.
package emailvalidator

import (
	"regexp"
	"strings"
)

var (
	alpha    = "[a-zA-Z]"
	alnum    = "[a-zA-Z0-9]"
	alnumhy  = "[a-zA-Z0-9-]"
	atomChar = "[a-zA-Z0-9!#$%&'*+/=?^_{|}~.\"`-]"

	// (?: ... ) はグループ化だがキャプチャしない表記

	// Defines the local part of an email address (before '@')
	// The first character must be alphanumeric or a special character
	// The following characters may include dots (".") but cannot be consecutive ("..")
	// 先頭は英数字または記号でその後は.を含む英数字または記号
	localPart = "(?:" + atomChar + "(?:\\." + atomChar + ")*){1,64}"

	// Domain validation rules:
	// - Each domain label consists of alphanumeric characters and hyphens (-)
	// - A hyphen (-) cannot appear at the beginning or the end of a label
	// - Each label must be between 1 and 63 characters
	// ドメイン部分
	// ドメインは英数字と-が使用できる,しかし-は許可されるが最初と最後にはつかってはいけない
	// Domain validation
	// ドメインの各パートは1~63文字,最初と最後に-は使えない
	hostLabelPattern = alnum + "(?:" + alnumhy + "{0,61}" + alnum + ")?"
	// Top-Level Domain (TLD) validation rules:
	// - Must start with an alphabetic character
	// - Can contain up to 63 characters, with alphanumeric characters only
	//　tldは先頭英字が必須で最大63文字
	tldLabelPattern = alpha + alnum + "{0,62}"
	// Email addresses can use an IP address as the domain part
	// The IP address must be enclosed in square brackets [ ]
	// - IPv6 format: "IPv6:" followed by hexadecimal characters and colons
	// - IPv4 format: Four octets (0-255) separated by dots (.)
	// メアドのドメイン部分にIPアドレス表記が可能
	// ipは[]で囲む
	// []の中はipv6またはipv4のどちらか
	// ipv6の正規表現: IPv6:[a-fA-F0-9:]+
	// ipv4の正規表現: \d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}
	addressLiteral = "\\[(?:IPv6:[a-fA-F0-9:]+|\\d{1,3}\\.\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\\]"
	// Defines the domain part of an email address
	// - It can be either an IP address or a traditional domain name
	// - Domain names consist of multiple host labels separated by dots (.)
	// - The last label (TLD) is treated separately and is optional
	// ipアドレス表記 or 単数のドメインパート or 複数ドメインパート(複数パートある場合、最後のパートはTLDとして扱う)
	domainPartPattern = "(?:" + addressLiteral + "|" + hostLabelPattern + "|(?:" + hostLabelPattern + "(?:\\." + hostLabelPattern + ")*\\." + tldLabelPattern + "))"

	// Compiled regular expression to validate the overall email format
	// - Uses \A and \z to ensure the entire string matches the pattern
	// - Avoids issues with newlines affecting validation
	// 改行の影響を避けるために\Aと\zを使う
	rfcRegex = regexp.MustCompile("\\A" + localPart + "(?:@" + domainPartPattern + ")?\\z")
)

/*
Validates an email address based on RFC 5322-like rules.

Parameters:
  - email (string): The email address to validate.

Returns:
  - bool: true if the email is valid, false otherwise.

This function does NOT strictly conform to RFC 5322 but provides practical validation.

IsValid 与えられたメールアドレスがRFC形式に従っているかどうかをチェックします.
この判定は厳密にRFC5322に準拠しているかを判定するものではことに注意してください.
*/
func IsValid(email string) bool {
	if !rfcRegex.MatchString(email) {
		return false
	}

	// `.`がローカルパートの先頭と末尾の `.`に存在している or 連続している場合はinvalid
	parts := strings.Split(email, "@")
	if len(parts) > 2 {
		return false
	}

	local := parts[0]

	if strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") {
		return false
	}

	if strings.Contains(local, "..") {
		return false
	}

	if len(parts) == 2 && len(parts[1]) > 255 {
		return false
	}

	return true
}
