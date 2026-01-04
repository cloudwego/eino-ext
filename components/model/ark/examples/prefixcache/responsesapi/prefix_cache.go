/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/cloudwego/eino-ext/components/model/ark"
)

func main() {
	ctx := context.Background()

	// Get ARK_API_KEY and ARK_MODEL_ID: https://www.volcengine.com/docs/82379/1399008
	responsesAPIChatModel, err := ark.NewResponsesAPIChatModel(ctx, &ark.ResponsesAPIConfig{
		APIKey: os.Getenv("ARK_API_KEY"),
		Model:  os.Getenv("ARK_MODEL_ID"),
	})

	if err != nil {
		log.Fatalf("NewResponsesAPIChatModel failed, err=%v", err)
	}

	tools := []*schema.ToolInfo{
		{
			Name: "article_content_extractor",
			Desc: "Extract key statements and chapter summaries from the provided article content",
			ParamsOneOf: schema.NewParamsOneOfByParams(
				map[string]*schema.ParameterInfo{
					"content": {
						Type:     schema.String,
						Desc:     "The full article content to analyze and extract key information from",
						Required: true,
					},
				}),
		},
	}

	// create response prefix cache, note: more than 1024 tokens are required, otherwise the prefix cache cannot be created
	cacheInfo, err := responsesAPIChatModel.CreatePrefixCache(ctx, []*schema.Message{
		schema.SystemMessage(`Once upon a time, in a quaint little village surrounded by vast green forests and blooming meadows, there lived a spirited young girl known as Little Red Riding Hood. She earned her name from the vibrant red cape that her beloved grandmother had sewn for her, a gift that she cherished deeply. This cape was more than just a piece of clothing; it was a symbol of the bond between her and her grandmother, who lived on the other side of the great woods, near a sparkling brook that bubbled merrily all year round.

			One sunny morning, Little Red Riding Hood's mother called her into the cozy kitchen, where the aroma of freshly baked bread filled the air. “My dear,” she said, “your grandmother isn’t feeling well today. I want you to take her this basket of treats. There are some delicious cakes, a jar of honey, and her favorite herbal tea. Can you do that for me?”
			
			Little Red Riding Hood’s eyes sparkled with excitement as she nodded eagerly. “Yes, Mama! I’ll take good care of them!” Her mother handed her a beautifully woven basket, filled to the brim with goodies, and reminded her, “Remember to stay on the path and don’t talk to strangers.”
			
			“I promise, Mama!” she replied confidently, pulling her red hood over her head and setting off on her adventure. The sun shone brightly, and birds chirped merrily as she walked, making her feel like she was in a fairy tale.
			
			As she journeyed through the woods, the tall trees whispered secrets to one another, and colorful flowers danced in the gentle breeze. Little Red Riding Hood was so enchanted by the beauty around her that she began to hum a tune, her voice harmonizing with the sounds of nature.
			
			However, unbeknownst to her, lurking in the shadows was a cunning wolf. The wolf was known throughout the forest for his deceptive wit and insatiable hunger. He watched Little Red Riding Hood with keen interest, contemplating his next meal.
			
			“Good day, little girl!” the wolf called out, stepping onto the path with a friendly yet sly smile.
			
			Startled, she halted and took a step back. “Hello there! I’m just on my way to visit my grandmother,” she replied, clutching the basket tightly.
			
			“Ah, your grandmother! I know her well,” the wolf said, his eyes glinting with mischief. “Why don’t you pick some lovely flowers for her? I’m sure she would love them, and I’m sure there are many beautiful ones just off the path.”
			
			Little Red Riding Hood hesitated for a moment but was easily convinced by the wolf’s charming suggestion. “That’s a wonderful idea! Thank you!” she exclaimed, letting her curiosity pull her away from the safety of the path. As she wandered deeper into the woods, her gaze fixed on the vibrant blooms, the wolf took a shortcut towards her grandmother’s house.
			
			When the wolf arrived at Grandma’s quaint cottage, he knocked on the door with a confident swagger. “It’s me, Little Red Riding Hood!” he shouted in a high-pitched voice to mimic the girl.
			
			“Come in, dear!” came the frail voice of the grandmother, who had been resting on her cozy bed, wrapped in warm blankets. The wolf burst through the door, his eyes gleaming with the thrill of his plan.
			
			With astonishing speed, the wolf gulped down the unsuspecting grandmother whole. Afterward, he dressed in her nightgown, donning her nightcap and climbing into her bed. He lay there, waiting for Little Red Riding Hood to arrive, concealing his wicked smile behind a facade of innocence.
			
			Meanwhile, Little Red Riding Hood was merrily picking flowers, completely unaware of the impending danger. After gathering a beautiful bouquet of wildflowers, she finally made her way back to the path and excitedly skipped towards her grandmother’s cottage.
			
			Upon arriving, she noticed the door was slightly ajar. “Grandmother, it’s me!” she called out, entering the dimly lit home. It was silent, with only the faint sound of an old clock ticking in the background. She stepped into the small living room, a feeling of unease creeping over her.
			
			“Grandmother, are you here?” she asked, peeking into the bedroom. There, she saw a figure lying under the covers. 
			
			“Grandmother, what big ears you have!” she exclaimed, taking a few cautious steps closer.
			
			“All the better to hear you with, my dear,” the wolf replied in a voice that was deceptively sweet.
			
			“Grandmother, what big eyes you have!” Little Red Riding Hood continued, now feeling an unsettling chill in the air.
			
			“All the better to see you with, my dear,” the wolf said, his eyes narrowing as he tried to contain his glee.
			
			“Grandmother, what big teeth you have!” she exclaimed, the terror flooding her senses as she began to realize this was no ordinary visit.
			
			“All the better to eat you with!” the wolf roared, springing out of the bed with startling speed.
			
			Just as the wolf lunged towards her, a brave woodsman, who had been passing by the cottage and heard the commotion, burst through the door. His strong presence was a beacon of hope in the dire situation. “Stay back, wolf!” he shouted with authority, brandishing his axe.
			
			The wolf, taken aback by the sudden intrusion, hesitated for a moment. Before he could react, the woodsman swung his axe with determination, and with a swift motion, he drove the wolf away, rescuing Little Red Riding Hood and her grandmother from certain doom.
			
			Little Red Riding Hood was shaking with fright, but relief washed over her as the woodsman helped her grandmother out from behind the bed where the wolf had hidden her. The grandmother, though shaken, was immensely grateful to the woodsman for his bravery. “Thank you so much! You saved us!” she cried, embracing him warmly.
			
			Little Red Riding Hood, still in shock but filled with gratitude, looked up at the woodsman and said, “I promise I will never stray from the path again. Thank you for being our hero!”
			
			From that day on, the woodland creatures spoke of the brave woodsman who saved Little Red Riding Hood and her grandmother. Little Red Riding Hood learned a valuable lesson about being cautious and listening to her mother’s advice. The bond between her and her grandmother grew stronger, and they often reminisced about that day’s adventure over cups of tea, surrounded by cookies and laughter.
			
			To ensure safety, Little Red Riding Hood always took extra precautions when traveling through the woods, carrying a small whistle her grandmother had given her. It would alert anyone nearby if she ever found herself in trouble again.
			
			And so, in the heart of that small village, life continued, filled with love, laughter, and the occasional adventure, as Little Red Riding Hood and her grandmother thrived, forever grateful for the friendship of the woodsman who had acted as their guardian that fateful day.
			
			And they all lived happily ever after.
			
			The end.`),
	}, 300, model.WithTools(tools))
	if err != nil {
		log.Fatalf("CreatePrefixCache failed, err=%v", err)
	}

	// use cache information in subsequent requests
	cacheOpt := &ark.CacheOption{
		HeadPreviousResponseID: &cacheInfo.ResponseID,
	}

	outMsg, err := responsesAPIChatModel.Generate(ctx, []*schema.Message{
		schema.UserMessage("What is the main idea expressed above？"),
	}, ark.WithCache(cacheOpt))

	if err != nil {
		log.Fatalf("Generate failed, err=%v", err)
	}

	respID, ok := ark.GetResponseID(outMsg)
	if !ok {
		log.Fatalf("not found response id in message")
	}

	log.Printf("\ngenerate output: \n")
	log.Printf("  request_id: %s\n", respID)
	respBody, _ := json.MarshalIndent(outMsg, "  ", "  ")
	log.Printf("  body: %s\n", string(respBody))
}
func ptrOf[T any](v T) *T {
	return &v

}
