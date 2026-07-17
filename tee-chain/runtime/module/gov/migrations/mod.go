package migrations

func Migration(newVersion uint32) error {
	if newVersion == 14 {
		return V14()
	}

	return nil
}
