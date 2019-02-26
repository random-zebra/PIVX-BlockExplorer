package pivx

import (
   "encoding/binary"
   "io"
)

// Uint8 reads one byte from the provided reader and
// returns the resulting uint8.
func Uint8(r io.Reader) (uint8, error) {
   buf := make([]byte, 1)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := buf[0]
   return rv, nil
}

// Uint16 reads two bytes from the provided reader, converts it
// to a number using the provided byte order, and returns the resulting uint16.
func Uint16(r io.Reader, byteOrder binary.ByteOrder) (uint16, error) {
   buf := make([]byte, 2)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := byteOrder.Uint16(buf)
   return rv, nil
}

// Uint32 reads four bytes from the provided reader, converts it
// to a number using the provided byte order, and returns the resulting uint32.
func Uint32(r io.Reader, byteOrder binary.ByteOrder) (uint32, error) {
   buf := make([]byte, 4)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := byteOrder.Uint32(buf)
   return rv, nil
}

// Uint64 reads eight bytes from the provided reader, converts it
// to a number using the provided byte order, and returns the resulting uint64.
func Uint64(r io.Reader, byteOrder binary.ByteOrder) (uint64, error) {
   buf := make([]byte, 8)

   if _, err := io.ReadFull(r, buf); err != nil {
      return 0, err
   }

   rv := byteOrder.Uint64(buf)
   return rv, nil
}
