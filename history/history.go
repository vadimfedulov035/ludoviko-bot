package history


import (
    "os"
    "log"
    "encoding/json"
)


func LoadHistory(conf string) map[int64]map[string]string {
    history := make(map[int64]map[string]string)

    file, err := os.OpenFile(conf, os.O_RDONLY|os.O_CREATE, 0644)
    if err != nil {
        log.Printf("[OS error] History file opening: %v", err)
        return history
    }
    defer file.Close()

    data, err := os.ReadFile(conf)
    if err != nil {
        log.Printf("[OS error] History reading: %v", err)
        return history
    }

	if len(data) == 0 {
		log.Println("[OS error] History file is empty")
		return history
	}

    err = json.Unmarshal(data, &history)
    if err != nil {
        log.Printf("[OS error] History unmarshalling: %v", err)
		history = make(map[int64]map[string]string)
        return history
    }

	log.Println("[OS] History read")
    return history
}


func WriteHistory(conf string, history map[int64]map[string]string) {
    file, err := os.OpenFile(conf, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
    if err != nil {
        log.Printf("[OS error] History file opening: %v", err)
        return
    }
    defer file.Close()

    data, err := json.Marshal(history)
    if err != nil {
        log.Printf("[OS error] History marshalling: %v", err)
        return
    }

    _, err = file.Write(data)
    if err != nil {
        log.Printf("[OS error] History writing: %v", err)
    }

	log.Println("[OS] History written")
}

