package amqp

import (
	"bytes"
	"fmt"
)

// SASL Codes
const (
	CodeSASLOK      SASLCode = iota // Connection authentication succeeded.
	CodeSASLAuth                    // Connection authentication failed due to an unspecified problem with the supplied credentials.
	CodeSASLSys                     // Connection authentication failed due to a system error.
	CodeSASLSysPerm                 // Connection authentication failed due to a system error that is unlikely to be corrected without intervention.
	CodeSASLSysTemp                 // Connection authentication failed due to a transient system error.
)

type Type uint8

// Composite Types
const (
	TypeSASLMechanism Type = 0x40
	TypeSASLInit      Type = 0x41
	TypeSASLChallenge Type = 0x42
	TypeSASLResponse  Type = 0x43
	TypeSASLOutcome   Type = 0x44
)

// SASL Mechanisms
const (
	SASLMechanismPLAIN Symbol = "PLAIN"
)

type SASLCode int

func (s *SASLCode) UnmarshalBinary(r byteReader) error {
	return Unmarshal(r, (*int)(s))
}

func ConnSASLPlain(username, password string) ConnOpt {
	return func(c *Conn) error {
		if c.saslHandlers == nil {
			c.saslHandlers = make(map[Symbol]stateFunc)
		}
		c.saslHandlers[SASLMechanismPLAIN] = (&saslHandlerPlain{
			c:        c,
			username: username,
			password: password,
		}).init
		return nil
	}
}

type saslHandlerPlain struct {
	c        *Conn
	username string
	password string
}

func (h *saslHandlerPlain) init() stateFunc {
	saslInit, err := Marshal(&SASLInit{
		Mechanism:       "PLAIN",
		InitialResponse: []byte("\x00" + h.username + "\x00" + h.password),
		Hostname:        "",
	})
	if err != nil {
		h.c.err = err
		return nil
	}

	wr := bufPool.New().(*bytes.Buffer)
	wr.Reset()
	defer bufPool.Put(wr)

	writeFrame(wr, FrameTypeSASL, 0, saslInit)

	fmt.Printf("Writing: %# 02x\n", wr.Bytes())

	_, err = h.c.net.Write(wr.Bytes())
	if err != nil {
		h.c.err = err
		return nil
	}

	return h.c.saslOutcome
}

/*
<type name="sasl-init" class="composite" source="list" provides="sasl-frame">
    <descriptor name="amqp:sasl-init:list" code="0x00000000:0x00000041"/>
    <field name="mechanism" type="symbol" mandatory="true"/>
    <field name="initial-response" type="binary"/>
    <field name="hostname" type="string"/>
</type>
*/
type SASLInit struct {
	Mechanism       Symbol
	InitialResponse []byte
	Hostname        string
}

func (si *SASLInit) MarshalBinary() ([]byte, error) {
	return marshalComposite(TypeSASLInit, []field{
		{value: si.Mechanism, omit: false},
		{value: si.InitialResponse, omit: len(si.InitialResponse) == 0},
		{value: si.Hostname, omit: len(si.Hostname) == 0},
	}...)
}

/*
SASLMechanisms Frame

00 53 40 c0 0e 01 e0 0b 01 b3 00 00 00 05 50 4c 41 49 4e

0 - indicates decriptor
53 - smallulong (?)
40 - sasl-mechanisms (?)

// composites are always lists
c0 - list
0e - size 14 bytes
01 - 1 element

e0 - array
0b - size 11 bytes
01 - 1 element

b3 - sym32

00 00 00 05 - 5 charaters
50 - "P"
4c - "L"
41 - "A"
49 - "I"
4e - "N"
*/
type SASLMechanisms struct {
	Mechanisms []Symbol
}

func (sm *SASLMechanisms) UnmarshalBinary(r byteReader) error {
	return unmarshalComposite(r, TypeSASLMechanism,
		&sm.Mechanisms,
	)
}

type SASLOutcome struct {
	Code           SASLCode
	AdditionalData []byte
}

func (so *SASLOutcome) UnmarshalBinary(r byteReader) error {
	return unmarshalComposite(r, TypeSASLOutcome,
		&so.Code,
		&so.AdditionalData,
	)
}