package CloudForest

import (
	"fmt"
	"log"
	"math/rand"
	"sort"
	"strconv"
)

type SparseUint8Feature struct {
	NumData map[int]uint8
	Missing map[int]bool
	Name    string
}

//Append will parse and append a single value to the end of the feature. It is generally only used
//during data parseing.
func (f *SparseUint8Feature) Append(v string) {
	// fv, err := strconv.ParseFloat(v, 64)
	// if err != nil {
	// 	f.NumData = append(f.NumData, 0.0)
	// 	f.Missing = append(f.Missing, true)
	// 	return
	// }
	// f.NumData = append(f.NumData, float64(fv))
	// f.Missing = append(f.Missing, false)
}

func (f *SparseUint8Feature) Less(i int, j int) bool {
	return f.NumData[i] < f.NumData[j]
}

func (f *SparseUint8Feature) PutStr(i int, v string) {
	fv, err := strconv.ParseUint(v, 0, 8)
	if err != nil {
		f.Missing[i] = true
		return
	}
	f.NumData[i] = uint8(fv)
	if f.Missing[i] {
		f.Missing[i] = false
	}
}

func (f *SparseUint8Feature) NCats() int {
	return 0
}

func (f *SparseUint8Feature) GetName() string {
	return f.Name
}

func (f *SparseUint8Feature) Length() int {
	return len(f.Missing)
}

func (f *SparseUint8Feature) IsMissing(i int) bool {
	return f.Missing[i]
}

func (f *SparseUint8Feature) PutMissing(i int) {
	f.Missing[i] = true
}

func (f *SparseUint8Feature) Get(i int) float64 {
	return float64(f.NumData[i])
}

func (f *SparseUint8Feature) GetStr(i int) (value string) {
	if f.Missing[i] {
		return "NA"
	}
	return fmt.Sprintf("%v", f.NumData[i])
}

func (f *SparseUint8Feature) Put(i int, v float64) {
	f.NumData[i] = uint8(v)
	f.Missing[i] = false
}

func (f *SparseUint8Feature) GoesLeft(i int, splitter *Splitter) bool {
	return float64(f.NumData[i]) <= splitter.Value
}

func (f *SparseUint8Feature) Predicted(cases *[]int) float64 {
	return f.Mean(cases)
}

func (f *SparseUint8Feature) Norm(i int, v float64) float64 {
	d := float64(f.NumData[i]) - v
	return d * d
}

//Decode split builds a splitter from the numeric values returned by BestNumSplit or
//BestCatSplit. Numeric splitters are decoded to send values <= num left. Categorical
//splitters are decoded to send categorical values for which the bit in cat is 1 left.
func (f *SparseUint8Feature) DecodeSplit(codedSplit interface{}) (s *Splitter) {

	s = &Splitter{f.Name, true, codedSplit.(float64), nil}

	return
}

/*
BestSplit finds the best split of the features that can be achieved using
the specified target and cases. It returns a Splitter and the decrease in impurity.

allocs contains pointers to reusable structures for use while searching for the best split and should
be initialized to the proper size with NewBestSplitAlocs.
*/
func (f *SparseUint8Feature) BestSplit(target Target,
	cases *[]int,
	parentImp float64,
	leafSize int,
	allocs *BestSplitAllocs) (codedSplit interface{}, impurityDecrease float64) {

	*allocs.NonMissing = (*allocs.NonMissing)[0:0]
	*allocs.Right = (*allocs.Right)[0:0]

	for _, i := range *cases {
		if f.Missing[i] {
			*allocs.Right = append(*allocs.Right, i)
		} else {
			*allocs.NonMissing = append(*allocs.NonMissing, i)
		}
	}
	if len(*allocs.NonMissing) == 0 {
		return
	}
	nmissing := float64(len(*allocs.Right))
	total := float64(len(*cases))
	nonmissing := total - nmissing

	nonmissingparentImp := target.Impurity(allocs.NonMissing, allocs.Counter)

	missingimp := 0.0
	if nmissing > 0 {
		missingimp = target.Impurity(allocs.Right, allocs.Counter)
	}

	codedSplit, impurityDecrease = f.BestNumSplit(target, allocs.NonMissing, nonmissingparentImp, leafSize, allocs)

	if nmissing > 0 && impurityDecrease > minImp {
		impurityDecrease = parentImp + ((nonmissing*(impurityDecrease-nonmissingparentImp) - nmissing*missingimp) / total)
	}
	return

}

/*
BestNumSplit searches over the possible splits of cases that can be made with f
and returns the one that minimizes the impurity of the target and the impurity decrease.

It expects to be provided cases for which the feature is not missing.

It searches by sorting the cases by the potential splitter and then evaluating each "gap"
between cases with non equal value as a potential split.

allocs contains pointers to reusable structures for use while searching for the best split and should
be initialized to the proper size with NewBestSplitAlocs.
*/
func (f *SparseUint8Feature) BestNumSplit(target Target,
	cases *[]int,
	parentImp float64,
	leafSize int,
	allocs *BestSplitAllocs) (codedSplit interface{}, impurityDecrease float64) {

	impurityDecrease = minImp
	codedSplit = 0.0

	if len(*cases) > 2*leafSize {
		sorter := allocs.Sorter
		sorter.Feature = f
		sorter.Cases = *cases
		sort.Sort(sorter)

		// Note: timsort is slower for my test cases but could potentially be made faster by eliminating
		// repeated allocations

		for i := leafSize; i < (len(sorter.Cases) - leafSize); i++ {
			c := sorter.Cases[i]
			//skip cases where the next sorted case has the same value as these can't be split on
			if f.Missing[c] == true || f.NumData[c] == f.NumData[sorter.Cases[i+1]] {
				continue
			}

			/*		BUG there is a reallocation of a slice (not the underlying array) happening here in
					BestNumSplit accounting for a chunk of runtime. Tried copying data between *l and *r
					but it was slower.  */
			innerimp := parentImp - target.SplitImpurity(sorter.Cases[:i], sorter.Cases[i:], nil, allocs)

			if innerimp > impurityDecrease {
				impurityDecrease = innerimp
				codedSplit = float64(f.NumData[c])

			}

		}

	}

	return
}

/*
FilterMissing loops over the cases and appends them into filtered.
For most use cases filtered should have zero length before you begin as it is not reset
internally
*/
func (f *SparseUint8Feature) FilterMissing(cases *[]int, filtered *[]int) {
	for _, c := range *cases {
		if f.Missing[c] != true {
			*filtered = append(*filtered, c)
		}
	}

}

/*
SplitImpurity calculates the impurity of a split into the specified left and right
groups. This is defined as pLi*(tL)+pR*i(tR) where pL and pR are the probability of case going left or right
and i(tl) i(tR) are the left and right impurities.

Counter is only used for categorical targets and should have the same length as the number of categories in the target.
*/
func (target *SparseUint8Feature) SplitImpurity(l []int, r []int, m []int, allocs *BestSplitAllocs) (impurityDecrease float64) {
	// l := *left
	// r := *right
	nl := float64(len(l))
	nr := float64(len(r))
	nm := 0.0

	impurityDecrease = nl * target.Impurity(&l, nil)
	impurityDecrease += nr * target.Impurity(&r, nil)
	if m != nil {
		nm = float64(len(m))
		impurityDecrease += nm * target.Impurity(&m, nil)
	}

	impurityDecrease /= nl + nr + nm
	return
}

//UpdateSImpFromAllocs willl be called when splits are being built by moving cases from r to l as in learning from numerical variables.
//Here it just wraps SplitImpurity but it can be implemented to provide further optimization.
func (target *SparseUint8Feature) UpdateSImpFromAllocs(l []int, r []int, m []int, allocs *BestSplitAllocs, movedRtoL []int) (impurityDecrease float64) {
	return target.SplitImpurity(l, r, m, allocs)
}

//Impurity returns Gini impurity or mean squared error vs the mean for a set of cases
//depending on weather the feature is categorical or numerical
func (target *SparseUint8Feature) Impurity(cases *[]int, counter *[]int) (e float64) {

	m := target.Mean(cases)
	e = target.Error(cases, m)
	return

}

//Error returns the  Mean Squared error of the cases specified vs the predicted
//value. Only non missing cases are considered.
func (target *SparseUint8Feature) Error(cases *[]int, predicted float64) (e float64) {
	e = 0.0
	n := 0
	for _, i := range *cases {
		if !target.Missing[i] {
			d := predicted - float64(target.NumData[i])
			e += d * d
			n += 1
		}

	}
	e = e / float64(n)
	return

}

//Mean returns the mean of the feature for the cases specified
func (target *SparseUint8Feature) Mean(cases *[]int) (m float64) {
	m = 0.0
	n := 0
	for _, i := range *cases {
		if !target.Missing[i] {
			m += float64(target.NumData[i])
			n += 1
		}

	}
	m = m / float64(n)
	return

}

//Mode returns the mode category feature for the cases specified
func (f *SparseUint8Feature) Mode(cases *[]int) (m float64) {
	counts := make(map[float64]int, 4)
	for _, i := range *cases {
		if !f.Missing[i] {
			counts[float64(f.NumData[i])] += 1
		}

	}
	max := 0
	for k, v := range counts {
		if v > max {
			m = k
			max = v
		}
	}
	return

}

//Span returns the lengh along the real line spaned by the specified cases
func (f *SparseUint8Feature) Span(cases *[]int) (span float64) {
	first := true
	min := 0.0
	max := 0.0
	val := 0.0
	for _, i := range *cases {
		if !f.Missing[i] {
			val = float64(f.NumData[i])
			if first {
				min = val
				max = val
				continue
			}
			switch {
			case val > max:
				max = val
			case val < min:
				min = val
			}

		}

	}

	return max - min

}

//Find predicted takes the indexes of a set of cases and returns the
//predicted value. For categorical features this is a string containing the
//most common category and for numerical it is the mean of the values.
func (f *SparseUint8Feature) FindPredicted(cases []int) (pred string) {
	pred = fmt.Sprintf("%v", f.Mean(&cases))
	if pred == "NaN" {
		log.Print("NaN predicted with cases ", len(cases))
	}
	return

}

//Shuffle does an inplace shuffle of the specified feature
func (f *SparseUint8Feature) Shuffle() {
	capacity := len(f.Missing)
	//shuffle
	for j := 0; j < capacity; j++ {
		sourcei := j + rand.Intn(capacity-j)
		missing := f.Missing[j]
		f.Missing[j] = f.Missing[sourcei]
		f.Missing[sourcei] = missing

		data := f.NumData[j]
		f.NumData[j] = f.NumData[sourcei]
		f.NumData[sourcei] = data

	}

}

//ShuffleCases does an inplace shuffle of the specified cases
func (f *SparseUint8Feature) ShuffleCases(cases *[]int) {
	capacity := len(*cases)
	//shuffle
	for j := 0; j < capacity; j++ {

		targeti := (*cases)[j]
		sourcei := (*cases)[j+rand.Intn(capacity-j)]
		missing := f.Missing[targeti]
		f.Missing[targeti] = f.Missing[sourcei]
		f.Missing[sourcei] = missing

		data := f.NumData[targeti]
		f.NumData[targeti] = f.NumData[sourcei]
		f.NumData[sourcei] = data

	}

}

/*ShuffledCopy returns a shuffled version of f for use as an artificial contrast in evaluation of
importance scores. The new feature will be named featurename:SHUFFLED*/
func (f *SparseUint8Feature) ShuffledCopy() Feature {
	fake := f.Copy()
	fake.Shuffle()
	fake.(*SparseUint8Feature).Name += ":SHUFFLED"
	return fake

}

/*Copy returns a copy of f.*/
func (f *SparseUint8Feature) Copy() Feature {
	fake := &SparseUint8Feature{
		nil,
		make(map[int]bool),
		f.Name}
	//BUG: need to copy sparse maps
	// copy(fake.Missing, f.Missing)

	// fake.NumData = make(map[int]float64)
	// copy(fake.NumData, f.NumData)

	return fake
}

func (f *SparseUint8Feature) CopyInTo(copyf Feature) {
	//BUG: need to copy sparse maps
	// copy(copyf.(*SparseUint8Feature).Missing, f.Missing)
	// copy(copyf.(*SparseUint8Feature).NumData, f.NumData)
}

//ImputeMissing imputes the missing values in a feature to the mean or mode of the feature.
func (f *SparseUint8Feature) ImputeMissing() {
	cases := make([]int, 0, len(f.Missing))
	for i, _ := range f.Missing {
		cases = append(cases, i)
	}
	mean := 0.0

	mean = f.Mean(&cases)

	for i, m := range f.Missing {
		if m {

			f.NumData[i] = uint8(mean)

			f.Missing[i] = false

		}
	}
}