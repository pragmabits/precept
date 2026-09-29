module example.com/dependent

go 1.26.0

require example.com/library v0.0.0

replace example.com/library => ./library
