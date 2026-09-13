package dtemplate

import (
	"fmt"
	"math/rand/v2"
)

var adjectives = []string{"swift", "quiet", "brave", "lucky", "clever", "gentle", "bright", "bold"}
var nouns = []string{"otter", "falcon", "willow", "cedar", "comet", "harbor", "meadow", "raven"}

func GenerateName() string {
	adj := adjectives[rand.IntN(len(adjectives))]
	noun := nouns[rand.IntN(len(nouns))]
	return fmt.Sprintf("%s-%s-%d", adj, noun, rand.IntN(10000))
}
