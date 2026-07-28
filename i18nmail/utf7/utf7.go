// Copyright 2026 Tamás Gulácsi. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

// Package utf7 implements genuine (RFC2152) and modified UTF-7 encoding (RFC3501 section 5.1.3)
//
/*
# IMAP4rev1 Modified UTF-7 (5.1.3.  Mailbox International Naming Convention)

	By convention, international mailbox names in IMAP4rev1 are specified
	using a modified version of the UTF-7 encoding described in [UTF-7].
	Modified UTF-7 may also be usable in servers that implement an
	earlier version of this protocol.

	In modified UTF-7, printable US-ASCII characters, except for "&",
	represent themselves; that is, characters with octet values 0x20-0x25
	and 0x27-0x7e.  The character "&" (0x26) is represented by the
	two-octet sequence "&-".

	All other characters (octet values 0x00-0x1f and 0x7f-0xff) are
	represented in modified BASE64, with a further modification from
	[UTF-7] that "," is used instead of "/".  Modified BASE64 MUST NOT be
	used to represent any printing US-ASCII character which can represent
	itself.

	"&" is used to shift to modified BASE64 and "-" to shift back to
	US-ASCII.  There is no implicit shift from BASE64 to US-ASCII, and
	null shifts ("-&" while in BASE64; note that "&-" while in US-ASCII
	means "&") are not permitted.  However, all names start in US-ASCII,
	and MUST end in US-ASCII; that is, a name that ends with a non-ASCII
	ISO-10646 character MUST end with a "-").

	The purpose of these modifications is to correct the following
	problems with UTF-7:

	   1) UTF-7 uses the "+" character for shifting; this conflicts with
	      the common use of "+" in mailbox names, in particular USENET
	      newsgroup names.

	   2) UTF-7's encoding is BASE64 which uses the "/" character; this
	      conflicts with the use of "/" as a popular hierarchy delimiter.

	   3) UTF-7 prohibits the unencoded usage of "\"; this conflicts with
	      the use of "\" as a popular hierarchy delimiter.

	   4) UTF-7 prohibits the unencoded usage of "~"; this conflicts with
	      the use of "~" in some servers as a home directory indicator.

	   5) UTF-7 permits multiple alternate forms to represent the same
	      string; in particular, printable US-ASCII characters can be
	      represented in encoded form.

	   Although modified UTF-7 is a convention, it establishes certain
	   requirements on server handling of any mailbox name with an
	   embedded "&" character.  In particular, server implementations
	   MUST preserve the exact form of the modified BASE64 portion of a
	   modified UTF-7 name and treat that text as case-sensitive, even if
	   names are otherwise case-insensitive or case-folded.

	   Server implementations SHOULD verify that any mailbox name with an
	   embedded "&" character, used as an argument to CREATE, is: in the
	   correctly modified UTF-7 syntax, has no superfluous shifts, and
	   has no encoding in modified BASE64 of any printing US-ASCII
	   character which can represent itself.  However, client
	   implementations MUST NOT depend upon the server doing this, and
	   SHOULD NOT attempt to create a mailbox name with an embedded "&"
	   character unless it complies with the modified UTF-7 syntax.

	   Server implementations which export a mail store that does not
	   follow the modified UTF-7 convention MUST convert to modified
	   UTF-7 any mailbox name that contains either non-ASCII characters
	   or the "&" character.

	        For example, here is a mailbox name which mixes English,
	        Chinese, and Japanese text:
	        ~peter/mail/&U,BTFw-/&ZeVnLIqe-

	        For example, the string "&Jjo!" is not a valid mailbox
	        name because it does not contain a shift to US-ASCII
	        before the "!".  The correct form is "&Jjo-!".  The
	        string "&U,BTFw-&ZeVnLIqe-" is not permitted because it
	        contains a superfluous shift.  The correct form is
	        "&U,BTF2XlZyyKng-".

# UTF-7 Definition

	  A UTF-7 stream represents 16-bit Unicode characters using 7-bit US-
	  ASCII octets as follows:

	     Rule 1: (direct encoding) Unicode characters in set D above may be
	     encoded directly as their ASCII equivalents. Unicode characters in
	     Set O may optionally be encoded directly as their ASCII
	     equivalents, bearing in mind that many of these characters are
	     illegal in header fields, or may not pass correctly through some
	     mail gateways.

	     Rule 2: (Unicode shifted encoding) Any Unicode character sequence
	     may be encoded using a sequence of characters in set B, when
	     preceded by the shift character "+" (US-ASCII character value
	     decimal 43). The "+" signals that subsequent octets are to be
	     interpreted as elements of the Modified Base64 alphabet until a
	     character not in that alphabet is encountered. Such characters
	     include control characters such as carriage returns and line
	     feeds; thus, a Unicode shifted sequence always terminates at the
	     of a line. As a special case, if the sequence terminates with the
	     character "-" (US-ASCII decimal 45) then that character is
	     absorbed; other terminating characters are not absorbed and are
	     processed normally.

	     Note that if the first character after the shifted sequence is "-"
	     then an extra "-" must be present to terminate the shifted
	     sequence so that the actual "-" is not itself absorbed.

	     Rationale. A terminating character is necessary for cases where
	     the next character after the Modified Base64 sequence is part of
	     character set B or is itself the terminating character. It can
	     also enhance readability by delimiting encoded sequences.

	     Also as a special case, the sequence "+-" may be used to encode
	     the character "+". A "+" character followed immediately by any
	     character other than members of set B or "-" is an ill-formed
	     sequence.

	     Unicode is encoded using Modified Base64 by first converting
	     Unicode 16-bit quantities to an octet stream (with the most
	     significant octet first). Surrogate pairs (UTF-16) are converted
	     by treating each half of the pair as a separate 16 bit quantity
	     (i.e., no special treatment). Text with an odd number of octets is
	     ill-formed. ISO 10646 characters outside the range addressable via
	     surrogate pairs cannot be encoded.

	     Rationale. ISO/IEC 10646-1:1993(E) specifies that when characters
	     the UCS-2 form are serialized as octets, that the most significant
	     octet appear first.  This is also in keeping with common network
	     practice of choosing a canonical format for transmission.

	     Rationale. The policy for code point allocation within ISO 10646
	     and Unicode is that the repertoires be kept synchronized. No code
	     points will be allocated in ISO 10646 outside the range
	     addressable by surrogate pairs.

	     Next, the octet stream is encoded by applying the Base64 content
	     transfer encoding algorithm as defined in RFC 2045, modified to
	     omit the "=" pad character. Instead, when encoding, zero bits are
	     added to pad to a Base64 character boundary. When decoding, any
	     bits at the end of the Modified Base64 sequence that do not
	     constitute a complete 16-bit Unicode character are discarded. If
	     such discarded bits are non-zero the sequence is ill-formed.

	     Rationale. The pad character "=" is not used when encoding
	     Modified Base64 because of the conflict with its use as an escape
	     character for the Q content transfer encoding in RFC 2047 header
	     fields, as mentioned above.

	     Rule 3: The space (decimal 32), tab (decimal 9), carriage return
	     (decimal 13), and line feed (decimal 10) characters may be
	     directly represented by their ASCII equivalents. However, note
	     that MIME content transfer encodings have rules concerning the use
	     of such characters. Usage that does not conform to the
	     restrictions of RFC 822, for example, would have to be encoded
	     using MIME content transfer encodings other than 7bit or 8bit,
	     such as quoted-printable, binary, or base64.

	  Given this set of rules, Unicode characters which may be encoded via
	  rules 1 or 3 take one octet per character, and other Unicode
	  characters are encoded on average with 2 2/3 octets per character

	plus one octet to switch into Modified Base64 and an optional octet
	  to switch out.

	     Example. The Unicode sequence "A<NOT IDENTICAL TO><ALPHA>."
	     (hexadecimal 0041,2262,0391,002E) may be encoded as follows:

	           A+ImIDkQ.

	     Example. The Unicode sequence "Hi Mom -<WHITE SMILING FACE>-!"
	     (hexadecimal 0048, 0069, 0020, 004D, 006F, 006D, 0020, 002D, 263A,
	      002D, 0021) may be encoded as follows:

	           Hi Mom -+Jjo--!

	     Example. The Unicode sequence representing the Han characters for
	     the Japanese word "nihongo" (hexadecimal 65E5,672C,8A9E) may be
	     encoded as follows:

	           +ZeVnLIqe-
*/
package utf7

import (
	"encoding/base64"

	"golang.org/x/text/encoding"
)

const (
	min = 0x20 // Minimum self-representing UTF-7 value
	max = 0x7E // Maximum self-representing UTF-7 value
)

var (
	Modified = flavor{
		shiftChar: '&',
		b64Enc:    base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+,"),
	}
	Genuine = flavor{
		shiftChar: '+',
		b64Enc:    base64.NewEncoding("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"),
	}
	Encoding = Genuine
)

type flavor struct {
	b64Enc    *base64.Encoding
	shiftChar byte
}

func (F flavor) NewDecoder() *encoding.Decoder {
	return &encoding.Decoder{Transformer: &decoder{flavor: F}}
}

func (F flavor) NewEncoder() *encoding.Encoder {
	return &encoding.Encoder{Transformer: encoder{flavor: F}}
}

// Encoding is the modified UTF-7 encoding.
// var Encoding encoding.Encoding = enc{}
