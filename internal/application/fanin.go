// internal/application/fanin.go
package application

import (
	"sync"

	"marketflow/internal/domain"
)

type FanIn struct {
	inputs []<-chan domain.PriceUpdate
	output chan domain.PriceUpdate
	wg     sync.WaitGroup
}

func NewFanIn(inputs []<-chan domain.PriceUpdate) *FanIn {
	fanIn := &FanIn{
		inputs: inputs,
		output: make(chan domain.PriceUpdate, 1000),
	}

	// Запускаем горутину для каждого входного канала
	for _, input := range inputs {
		fanIn.wg.Add(1)
		go fanIn.merge(input)
	}

	// Закрываем выходной канал когда все входные закрыты
	go func() {
		fanIn.wg.Wait()
		close(fanIn.output)
	}()

	return fanIn
}

func (f *FanIn) merge(input <-chan domain.PriceUpdate) {
	defer f.wg.Done()
	
	for update := range input {
		select {
		case f.output <- update:
		default:
			// Если канал заполнен, пропускаем обновление
		}
	}
}

func (f *FanIn) Output() <-chan domain.PriceUpdate {
	return f.output
}