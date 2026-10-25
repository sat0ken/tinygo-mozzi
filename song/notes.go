package song

// MIDI note numbers for octaves 3 through 6, so a melody can be written
// the way each note is named on a staff (C4 is middle C = MIDI note 60).
// Need a wider range? The pattern is exactly -12/+12 per octave down/up.
const (
	C3  Note = 48
	Cs3 Note = 49 // = Db3
	D3  Note = 50
	Ds3 Note = 51 // = Eb3
	E3  Note = 52
	F3  Note = 53
	Fs3 Note = 54 // = Gb3
	G3  Note = 55
	Gs3 Note = 56 // = Ab3
	A3  Note = 57
	As3 Note = 58 // = Bb3
	B3  Note = 59

	C4  Note = 60 // middle C
	Cs4 Note = 61
	D4  Note = 62
	Ds4 Note = 63
	E4  Note = 64
	F4  Note = 65
	Fs4 Note = 66
	G4  Note = 67
	Gs4 Note = 68
	A4  Note = 69 // concert pitch A, 440Hz
	As4 Note = 70
	B4  Note = 71

	C5  Note = 72
	Cs5 Note = 73
	D5  Note = 74
	Ds5 Note = 75
	E5  Note = 76
	F5  Note = 77
	Fs5 Note = 78
	G5  Note = 79
	Gs5 Note = 80
	A5  Note = 81
	As5 Note = 82
	B5  Note = 83

	C6  Note = 84
	Cs6 Note = 85
	D6  Note = 86
	Ds6 Note = 87
	E6  Note = 88
	F6  Note = 89
	Fs6 Note = 90
	G6  Note = 91
	Gs6 Note = 92
	A6  Note = 93
	As6 Note = 94
	B6  Note = 95
)
